package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

var db *sqlx.DB
var client *redis.Client
var concurrentLimit chan byte

func init() {
	// Init redis
	fmt.Print("Connecting redis...")
	client = redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		//Password: "", // No password set
		DB: 2, // Use default DB
	})
	fmt.Println("DONE")

	// Init database
	fmt.Print("Connecting database...")
	dbTmp, err := sqlx.Connect("mysql", "19xxjjy:d4G8f6a4@tcp(127.0.0.1:3306)/19xxjjy")
	if err != nil {
		fmt.Printf("\n Error ibgImageRegn connecting to mysql: %v \n", err)
	} else {
		fmt.Println("DONE")
	}
	db = dbTmp

	// Init concurrent limit channel
	concurrentLimit = make(chan byte, 10)
}

func main() {
	fmt.Println("Waiting for message in the queue...")
	for {
		brPop := client.BRPop(time.Hour*24*365, "getter")
		result, err := brPop.Result()
		if err != nil {
			fmt.Printf("Error in getting results from brPop, %v \n", err)
			// Sleep 1s
			time.Sleep(time.Second)
			continue
		}

		fmt.Printf("Read data by using brPop: %v \n", result)
		msg := &getterMessage{}
		// The 0 index in the result is key name, the 1 index in the result is value
		if err = json.Unmarshal([]byte(result[1]), msg); err != nil {
			fmt.Printf("Error in unmarshal results from brPop, %v \n", err)
			continue
		}

		// Increase count in buffer
		concurrentLimit <- 1
		// msg is ok
		go func() {
			// Decrease count in buffer
			defer func() {
				<-concurrentLimit

				// Recover from panic
				if err := recover(); err != nil {
					fmt.Printf("RECOVERED: %v, %s\n", err, debug.Stack())
				}
			}()

			fmt.Printf("Time: %s \n", time.Now().Format("2006-01-02 03-04-05"))

			// RUN
			run(msg)
		}()
	}
}

func run(message *getterMessage) {
	// Create struct
	var getter getter
	switch message.Type {
	case videoTypeKwai:
		getter = &kwaiGetter{}
	case videoTypeTiktok:
		fallthrough
	default:
		getter = &tiktokGetter{}
	}

	// 1. Validate
	if err := getter.validate(message); err != nil {
		fmt.Printf("[%d][%d]Error in validate: %s \n", message.Id, message.Type, err)
		return
	}

	// 2. Get content
	content, err := getUrlContent(message.Url)
	if err != nil {
		fmt.Printf("[%d][%d]Error in getting content of url: %v \n", message.Id, message.Type, err)
		return
	}
	// Replace &#x starting with &x for passing html/parse escape
	reg, _ := regexp.Compile("&#x(.{4});")
	bodyStr := reg.ReplaceAllString(string(content), ";x$1;")
	// Init document
	document, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		fmt.Printf("[%d][%d]Error in parsing response: %v \n", message.Id, message.Type, err)
		return
	}

	// 3. Extract data
	if err = getter.extractData(document); err != nil {
		fmt.Printf("[%d][%d]Error in extracting data: %v \n", message.Id, message.Type, err)
		return
	}

	// 4. Save
	if err = getter.save(); err != nil {
		fmt.Printf("[%d][%d]Error in saving data: %v \n", message.Id, message.Type, err)
		return
	}
}
