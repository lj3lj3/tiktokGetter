package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"strings"
)

// The Getter for Kwai(快手)
type kwaiGetter struct {
	videoGetter
}

func (g *kwaiGetter) validate(message *getterMessage) error {
	g.videoGetter.init(message)

	urlTmp, err := url.Parse(message.Url)
	if err != nil {
		return err
	}
	if urlTmp.Host != "m.gifshow.com" && urlTmp.Host != "live.kuaishou.com" {
		return fmt.Errorf("url is not from kwai: %s", urlTmp.Host)
	}

	return nil
}

func (g *kwaiGetter) extractData(document *goquery.Document) error {
	// Get like count
	g.data["likes"] = document.Find(".player-info-bar .video-info .like>p").First().Text()

	// Get comment count
	g.data["comments"] = document.Find(".player-info-bar .video-info .comment>p").First().Text()

	// Get user info
	userInfo, err := g.getUserInfo(document)
	if err != nil {
		return err
	}

	// Get video info
	videoInfo, err := g.getVideoInfo(document)
	if err != nil {
		return err
	}

	mergeMaps(g.data, userInfo, videoInfo)

	return nil
}

func (g *kwaiGetter) getVideoInfo(document *goquery.Document) (map[string]string, error) {
	data := make(map[string]string)

	// Get video title
	videoTitle, exists := document.Find("#video-player").Attr("alt")
	data["video_title"] = videoTitle

	// Get video poster
	style, exists := document.Find("#video-box>.poster").Attr("style")
	if exists {
		// Load poster
		matched := bgImageReg.FindStringSubmatch(style)
		if len(matched) == 0 {
			fmt.Printf("[%d][%d]Errror in finding video poster, style: %s \n", g.message.Id, g.message.Type, style)
			data["video_poster"] = ""
		} else {
			// Download video poster
			filePath, err := downloadFile(matched[1], fmt.Sprintf("%d_%d_video_poster", g.message.Id, g.message.Type))
			if err != nil {
				return nil, err
			}
			data["video_poster"] = filePath
		}
	}

	// Get video
	src, exists := document.Find("#video-player").Attr("src")
	if exists {
		// Download video
		filePath, err := downloadFile(src, fmt.Sprintf("%d_%d_video", g.message.Id, g.message.Type))
		if err != nil {
			return nil, err
		}
		data["video"] = filePath
	}

	return data, nil
}

func (g *kwaiGetter) getUserInfo(document *goquery.Document) (map[string]string, error) {
	data := make(map[string]string)
	userNode := document.Find(".player-info-bar .user")

	// Get user avatar
	style, exists := userNode.Find(".avatar").Attr("style")
	if exists {
		// Load avatar
		matched := bgImageReg.FindStringSubmatch(style)
		if len(matched) == 0 {
			fmt.Printf("[%d][%d]Errror in finding user avatar, style: %s \n", g.message.Id, g.message.Type, style)
			data["user_avatar"] = ""
		} else {
			data["user_avatar"] = matched[1]
		}
	}
	// Get user name
	data["user_name"] = userNode.Find(".info>.name").Text()
	// Get user id
	userIdStr := userNode.Find(".info>.txt").Text()
	// Remove useless character and trim space
	data["user_id"] = strings.TrimSpace(userIdStr[3:])

	fmt.Printf("[%d][%d]User info: %s \n", g.message.Id, g.message.Type, data)

	return data, nil
}

func (g *kwaiGetter) save() error {
	// Write back to database
	result, err := db.NamedExec("UPDATE table SET kwai_likes = :likes, kwai_comments = :comments,"+
		"kwai_user_name = :user_name, kwai_user_id = :user_id, kwai_user_avatar = :user_avatar, kwai_video_poster = :video_poster, "+
		"kwai_video = :video, kwai_video_title = :video_title WHERE id = :id", map[string]interface{}{
		"table":        g.table,
		"likes":        g.data["likes"],
		"user_name":    g.data["user_name"],
		"user_id":      g.data["user_id"],
		"user_avatar":  g.data["user_avatar"],
		"video_poster": g.data["video_poster"],
		"video":        g.data["video"],
		"video_title":  g.data["video_title"],
		"comments":     g.data["comments"],
		"id":           g.message.Id,
	})

	if err != nil {
		return err
	}
	_, err = result.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}
