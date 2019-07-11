package main

import "github.com/PuerkitoBio/goquery"

type getter interface {
	validate(message *getterMessage) error
	extractData(document *goquery.Document) error
	save() error
}

type getterMessage struct {
	Id   int       `json:"id"`
	Type videoType `json:"type"`
	Url  string    `json:"url"`
}

type videoType int8

const videoTypeTiktok videoType = 1
const videoTypeKwai videoType = 2
