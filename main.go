package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"io/ioutil"
	"net/url"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

type tiktokMessage struct {
	Id  int    `json:"id"`
	Url string `json:"url"`
}

var db *sqlx.DB
var client *redis.Client
var concurrentLimit chan byte
var bgImageReg *regexp.Regexp

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

	bgImageReg, _ = regexp.Compile(`background-image:url\((.+)\)`)
}

func main() {
	fmt.Println("waiting for message in the queue...")
	for {
		brPop := client.BRPop(time.Hour*24*365, "tiktokGetter")
		result, err := brPop.Result()
		if err != nil {
			fmt.Printf("error in getting results from brPop, %v \n", err)
			// sleep 1s
			time.Sleep(time.Second)
			continue
		}

		fmt.Printf("read data by using brPop: %v \n", result)
		msg := &tiktokMessage{}
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

				// Recover from panic
				if err := recover(); err != nil {
					fmt.Printf("RECOVERED: %v, %s\n", err, debug.Stack())
				}
			}()
			getTiktokData(msg)
		}()
	}
}

func getTiktokData(message *tiktokMessage) {
	// validate message
	urlTmp, err := url.Parse(message.Url)
	if err != nil {
		fmt.Printf("[%d]error when parsing urlTmp: %s \n", message.Id, err)
		return
	} else if urlTmp.Host != "v.douyin.com" && urlTmp.Host != "www.iesdouyin.com" {
		fmt.Printf("[%d]url is not from tictok: %s \n", message.Id, urlTmp.Host)
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

	// Init document
	document, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		fmt.Printf("[%d]error in parsing response: %v \n", message.Id, err)
		return
	}

	data := make(map[string]string)
	// Get like count
	likeCount, err := getCount(document, message, ".info-like>.count>i")
	if err != nil {
		return
	}
	data["like"] = strconv.Itoa(likeCount)

	// Get comment count
	commentCount, err := getCount(document, message, ".info-comment>.count>i")
	if err != nil {
		return
	}
	data["comment"] = strconv.Itoa(commentCount)

	// Get user info
	userInfo, err := getUserInfo(document, message)
	if err != nil {
		return
	}

	// Get video info
	videoInfo, err := getVideoInfo(document, message)
	if err != nil {
		return
	}

	// write back to database
	writeTiktokData(message, mergeMaps(data, userInfo, videoInfo))
}

func getVideoInfo(document *goquery.Document, message *tiktokMessage) (map[string]string, error) {
	data := make(map[string]string)

	// Get video title
	data["video_title"] = document.Find("#videoUser>.user-title").Text()

	// Get video poster
	style, exists := document.Find("#videoPoster").Attr("style")
	if exists {
		// Load poster
		matched := bgImageReg.FindStringSubmatch(style)
		if len(matched) == 0 {
			fmt.Printf("[%d]errror in finding video poster, style: %s \n", message.Id, style)
			data["video_poster"] = ""
		} else {
			// Download video poster
			filePath, err := downloadFile(matched[1], strconv.Itoa(message.Id)+"_video_poster")
			if err != nil {
				return nil, err
			}
			data["video_poster"] = filePath
		}
	}

	// Get video
	src, exists := document.Find("#theVideo").Attr("src")
	if exists {
		// Download video
		filePath, err := downloadFile(src, strconv.Itoa(message.Id)+"_video")
		if err != nil {
			return nil, err
		}
		data["video"] = filePath
	}

	return data, nil
}

func getUserInfo(document *goquery.Document, message *tiktokMessage) (map[string]string, error) {
	data := make(map[string]string)
	videoUserNode := document.Find("#videoUser")

	// Get user avatar
	style, exists := videoUserNode.Find(".user-avator").Attr("style")
	if exists {
		// Load avatar
		matched := bgImageReg.FindStringSubmatch(style)
		if len(matched) == 0 {
			fmt.Printf("[%d]errror in finding user avatar, style: %s \n", message.Id, style)
			data["user_avatar"] = ""
		} else {
			data["user_avatar"] = matched[1]
		}
	}
	// Get user name
	data["user_name"] = videoUserNode.Find(".user-info>.user-info-name").Text()
	// Get tiktok id
	// EDIT: tiktok id can be changed to letter
	tiktokIdHtml, err := videoUserNode.Find(".user-info>.user-info-id").Html()
	if err != nil {
		fmt.Printf("[%d]errror in getting html of user tiktok id: %v \n", message.Id, err)
	}
	// Find and replace
	reg, _ := regexp.Compile(`<i class="icon iconfont\s?">\s?;(.{6})\s?</i>`)
	tiktokIdStr := reg.ReplaceAllStringFunc(tiktokIdHtml, func(s string) string {
		matches := reg.FindStringSubmatch(s)
		if len(matches) > 1 {
			return strconv.Itoa(getDigitFromFontString("&#" + matches[1])) // Replace i DOM node with right number
		}
		return s
	})
	// Remove useless character and trim space
	data["user_tiktok_id"] = strings.TrimSpace(strings.Replace(tiktokIdStr, "抖音ID:", "", 1))

	fmt.Printf("[%d]user info: %s \n", message.Id, data)

	return data, nil
}

func getCount(document *goquery.Document, message *tiktokMessage, selector string) (int, error) {
	countStr := ""
	document.Find(selector).Each(func(i int, selection *goquery.Selection) {
		countStr += strconv.Itoa(getDigitFromFontString("&#" + selection.Text()[2:]))
	})
	// parse to int
	count, err := strconv.Atoi(countStr)
	if err != nil {
		fmt.Printf("[%d]error in parsing count to int: %v \n", message.Id, err)
		return 0, err
	}

	fmt.Printf("[%d]%s, count: %d \n", message.Id, selector, count)
	return count, nil
}

func getUrlContent(url string) ([]byte, error) {
	response, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func writeTiktokData(message *tiktokMessage, data map[string]string) {
	result, err := db.NamedExec("UPDATE df_signup SET likes = :likeCount, comments = :commentCount,"+
		"user_name = :userName, user_tiktok_id = :userTiktokId, user_avatar = :userAvatar, video_poster = :videoPoster, "+
		"video = :video, video_title = :videoTitle WHERE id = :id", map[string]interface{}{
		"likeCount":    data["like"],
		"userName":     data["user_name"],
		"userTiktokId": data["user_tiktok_id"],
		"userAvatar":   data["user_avatar"],
		"videoPoster":  data["video_poster"],
		"video":        data["video"],
		"videoTitle":   data["video_title"],
		"commentCount": data["comment"],
		"id":           message.Id,
	})
	if err != nil {
		fmt.Printf("[%d]error in updating database record: %v \n", message.Id, err)
	}
	_, err = result.RowsAffected()
	if err != nil {
		fmt.Printf("[%d]error in updating database record: %v \n", message.Id, err)
	}
}
