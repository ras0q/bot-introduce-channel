package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	traq "github.com/traPtitech/go-traq"
)

var (
	accessToken = os.Getenv("TRAQ_ACCESS_TOKEN")
	introChID   = os.Getenv("TRAQ_INTRO_CHANNNEL_ID")
	cli         = traq.NewAPIClient(traq.NewConfiguration())
	auth        = context.WithValue(context.Background(), traq.ContextAccessToken, accessToken)
)

func main() {
	c := cron.New(
		cron.WithLocation(time.FixedZone("Asia/Tokyo", 9*60*60)),
		cron.WithChain(cron.Recover(cron.DefaultLogger)),
	)

	if _, err := c.AddFunc("0 8,14,22 * * *", introduceChannel); err != nil {
		panic(err)
	}

	c.Start()

	forever := make(chan struct{})
	<-forever
}

func introduceChannel() {
	var (
		todayCh  traq.Channel
		fullPath string
	)

	chMap := getChannelsMap()

	// get a channel at random
	for _, ch := range chMap {
		if ch.GetArchived() || ch.GetForce() {
			continue
		}

		todayCh = ch

		break
	}

	fullPath = todayCh.GetName()
	_ch := todayCh

	// get full path
	for {
		if pid, ok := _ch.GetParentIdOk(); ok && pid != nil {
			if parent, ok := chMap[*pid]; ok {
				fullPath = parent.GetName() + "/" + fullPath
				_ch = parent
			}
		} else {
			fullPath = "#" + fullPath

			break
		}
	}

	topic := todayCh.GetTopic()
	if len(topic) > 0 {
		topic = "> " + strings.ReplaceAll(topic, "\n", "\n> ") + "\n"
	}

	cid := todayCh.GetId()
	subs := getSubscriversNumStr(cid)
	msgs, talkers := getMsgsAndTalkersNumStr(cid)
	pins := getPinsNumStr(cid)
	msg := fmt.Sprintf(
		`きなのがチャンネルを紹介するやんね！
## %s
%s
|説明|数|
|-|-:|
|メンバー数|%s|
|総メッセージ数|%s|
|会話に参加したユーザー数|%s|
|ピン止め数|%s|`,
		fullPath,
		topic,
		subs,
		msgs,
		talkers,
		pins,
	)

	postMessage(introChID, msg, true)
}

func getChannelsMap() map[string]traq.Channel {
	chs, res, err := cli.ChannelApi.
		GetChannels(auth).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	chMap := make(map[string]traq.Channel)
	for _, ch := range chs.Public {
		chMap[ch.GetId()] = ch
	}

	return chMap
}

func getSubscriversNumStr(id string) string {
	subs, res, err := cli.ChannelApi.
		GetChannelSubscribers(auth, id).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	return strconv.Itoa(len(subs))
}

func getMsgsAndTalkersNumStr(id string) (string, string) {
	stats, res, err := cli.ChannelApi.
		GetChannelStats(auth, id).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	return strconv.Itoa(int(stats.TotalMessageCount)), strconv.Itoa(len(stats.Users))
}

func getPinsNumStr(id string) string {
	pins, res, err := cli.ChannelApi.
		GetChannelPins(auth, id).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	return strconv.Itoa(len(pins))
}

func postMessage(chid string, msg string, embed bool) {
	if len(chid) == 0 {
		fmt.Println(msg)

		return
	}

	_, res, err := cli.MessageApi.
		PostMessage(auth, chid).
		PostMessageRequest(traq.PostMessageRequest{
			Content: msg,
			Embed:   &embed,
		}).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}
}
