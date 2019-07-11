package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// The Getter for Tiktok(抖音)
type tiktokGetter struct {
	videoGetter
}

func (g *tiktokGetter) validate(message *getterMessage) error {
	g.videoGetter.init(message)

	urlTmp, err := url.Parse(message.Url)
	if err != nil {
		return err
	}
	if urlTmp.Host != "v.douyin.com" && urlTmp.Host != "www.iesdouyin.com" {
		return fmt.Errorf("url is not from tiktok: %s", urlTmp.Host)
	}

	return nil
}

func (g *tiktokGetter) extractData(document *goquery.Document) error {
	// Get like count
	likeCount, err := g.getCount(document, ".info-like>.count>i")
	if err != nil {
		return err
	}
	g.data["like"] = strconv.Itoa(likeCount)

	// Get comment count
	commentCount, err := g.getCount(document, ".info-comment>.count>i")
	if err != nil {
		return err
	}
	g.data["comment"] = strconv.Itoa(commentCount)

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

func (g *tiktokGetter) save() error {
	// Write back to database
	result, err := db.NamedExec("UPDATE :table SET tiktok_likes = :likeCount, tiktok_comments = :commentCount,"+
		"tiktok_user_name = :userName, tiktok_user_id = :userTiktokId, tiktok_user_avatar = :userAvatar, tiktok_video_poster = :videoPoster, "+
		"tiktok_video = :video, tiktok_video_title = :videoTitle WHERE id = :id", map[string]interface{}{
		"table":        g.table,
		"likeCount":    g.data["like"],
		"userName":     g.data["user_name"],
		"userTiktokId": g.data["user_tiktok_id"],
		"userAvatar":   g.data["user_avatar"],
		"videoPoster":  g.data["video_poster"],
		"video":        g.data["video"],
		"videoTitle":   g.data["video_title"],
		"commentCount": g.data["comment"],
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

func (g *tiktokGetter) getVideoInfo(document *goquery.Document) (map[string]string, error) {
	data := make(map[string]string)

	// Get video title
	data["video_title"] = document.Find("#videoUser>.user-title").Text()

	// Get video poster
	style, exists := document.Find("#videoPoster").Attr("style")
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
	src, exists := document.Find("#theVideo").Attr("src")
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

func (g *tiktokGetter) getUserInfo(document *goquery.Document) (map[string]string, error) {
	data := make(map[string]string)
	videoUserNode := document.Find("#videoUser")

	// Get user avatar
	style, exists := videoUserNode.Find(".user-avator").Attr("style")
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
	data["user_name"] = videoUserNode.Find(".user-info>.user-info-name").Text()
	// Get tiktok id
	// EDIT: tiktok id can be changed to letter
	tiktokIdHtml, err := videoUserNode.Find(".user-info>.user-info-id").Html()
	if err != nil {
		fmt.Printf("[%d][%d]Errror in getting html of user tiktok id: %v \n", g.message.Id, g.message.Type, err)
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

	fmt.Printf("[%d][%d]User info: %s \n", g.message.Id, g.message.Type, data)

	return data, nil
}

func (g *tiktokGetter) getCount(document *goquery.Document, selector string) (int, error) {
	countStr := ""
	document.Find(selector).Each(func(i int, selection *goquery.Selection) {
		countStr += strconv.Itoa(getDigitFromFontString("&#" + selection.Text()[2:]))
	})
	// parse to int
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0, err
	}

	//fmt.Printf("[%d][%d]%s, count: %d \n", g.message.Id, g.message.Type, selector, count)
	return count, nil
}
