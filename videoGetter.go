package main

import (
	"github.com/PuerkitoBio/goquery"
	"regexp"
)

type videoGetter struct {
	data    map[string]string
	message *getterMessage
	table   string
}

func (g *videoGetter) init(message *getterMessage) {
	g.message = message
	g.data = make(map[string]string)
	g.table = "df_person"
}

func (g *videoGetter) validate(message *getterMessage) error {
	return nil
}

func (g *videoGetter) extractData(document *goquery.Document) error {
	return nil
}

func (g *videoGetter) save() error {
	return nil
}

var bgImageReg *regexp.Regexp

func init() {
	bgImageReg, _ = regexp.Compile(`background-image:url\((.+)\)`)
}
