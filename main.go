package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type likeMessage struct {
	Id  int    `json:"id"`
	Url string `json:"url"`
}

var db *sqlx.DB
var client *redis.Client
var concurrentLimit chan byte

func init() {
	// init redis
	fmt.Print("connecting redis...")
	client = redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		//Password: "", // no password set
		DB: 2, // use default DB
	})
	fmt.Println("DONE")

	// init database
	fmt.Print("connecting database...")
	dbTmp, err := sqlx.Connect("mysql", "bm201906:S5H2c4Y8@tcp(127.0.0.1:3306)/bm201906")
	if err != nil {
		fmt.Printf("\nerror in connecting to mysql: %v \n", err)
	} else {
		fmt.Println("DONE")
	}
	db = dbTmp

	// init concurrent limit channel
	concurrentLimit = make(chan byte, 10)
}

func main() {
	fmt.Println("waiting for message in the queue...")
	for {
		brPop := client.BRPop(time.Hour*24*365, "likeGetter")
		result, err := brPop.Result()
		if err != nil {
			fmt.Printf("error in getting results from brPop, %v \n", err)
			// sleep 1s
			time.Sleep(time.Second)
			continue
		}

		fmt.Printf("read data by using brPop: %v \n", result)
		msg := &likeMessage{}
		// the 0 index in the result is key name, the 1 index in the result is value
		if err = json.Unmarshal([]byte(result[1]), msg); err != nil {
			fmt.Printf("error in unmarshal results from brPop, %v \n", err)
			continue
		}

		// increase count in buffer
		concurrentLimit <- 1
		// msg is ok
		go func() {
			// decrease count in buffer
			defer func() {
				<-concurrentLimit
			}()
			getLikeCount(msg)
		}()
	}
}

func getLikeCount(message *likeMessage) {
	// validate message
	urlTmp, err := url.Parse(message.Url)
	if err != nil {
		fmt.Printf("[%d]error when parsing urlTmp: %s \n", message.Id, err)
		return
	} else if urlTmp.Host != "v.douyin.com" && urlTmp.Host != "www.iesdouyin.com" {
		fmt.Printf("[%d]url is not from douyin: %s \n", message.Id, urlTmp.Host)
		return
	}

	// urlTmp is ok
	content, err := getUrlContent(message.Url)
	if err != nil {
		fmt.Printf("[%d]error in getting content of url: %v \n", message.Id, err)
		return
	}

	// replace &#x starting with &x for passing html/parse escape
	reg, _ := regexp.Compile("&#x(.{4});")
	bodyStr := reg.ReplaceAllString(string(content), ";x$1;")

	// get like count from body
	document, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		fmt.Printf("[%d]error in parsing response: %v \n", message.Id, err)
		return
	}
	countStr := ""
	document.Find(".info-like>.count>i").Each(func(i int, selection *goquery.Selection) {
		countStr += strconv.Itoa(getDigitFromFontString("&#" + selection.Text()[2:]))
	})
	// parse to int
	count, err := strconv.Atoi(countStr)
	if err != nil {
		fmt.Printf("[%d]error in parsing count to int: %v \n", message.Id, err)
		return
	}

	fmt.Printf("[%d]like count: %d \n", message.Id, count)

	// write back to database
	writeLikeCount(message, count)
}

func getUrlContent(url string) ([]byte, error) {
	// ready to get url content
	var client = &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// set up header, let server treat us as mobile browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1")

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func writeLikeCount(message *likeMessage, likeCount int) {
	result, err := db.NamedExec("UPDATE df_signup SET likes = :likeCount WHERE id = :id", map[string]interface{}{
		"likeCount": likeCount,
		"id":        message.Id,
	})
	if err != nil {
		fmt.Printf("[%d]error in updating database record: %v \n", message.Id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected <= 0 {
		fmt.Printf("[%d]error in updating database record: %v \n", message.Id, err)
	}
}
