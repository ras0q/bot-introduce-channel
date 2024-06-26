package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
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
	re          = regexp.MustCompile("^#(gps|other|random|ramen)")
)

func main() {
	c := cron.New(
		cron.WithLocation(time.FixedZone("Asia/Tokyo", 9*60*60)),
		cron.WithChain(cron.Recover(cron.DefaultLogger)),
	)

	if _, err := c.AddFunc("0 8,13,18,23 * * *", introduceChannel); err != nil {
		panic(err)
	}

	c.Start()

	forever := make(chan struct{})
	<-forever
}

func introduceChannel() {
	var (
		todayCh    traq.Channel
		fullPath   string
		latestMsgs []traq.Message
	)

	chMap := getChannelsMap()
	userMap := getUsersMap()

	// get a channel at random
	for _, ch := range chMap {
		if ch.GetArchived() || ch.GetForce() {
			continue
		}

		fp := getFullPath(ch, chMap)

		if re.MatchString(fp) {
			msgs := getLatestMsgs(ch)
			if len(msgs) < 3 {
				continue
			}

			slices.Reverse(msgs)

			todayCh = ch
			fullPath = fp
			latestMsgs = msgs

			break
		}
	}

	topic := todayCh.GetTopic()
	if len(topic) > 0 {
		topic = "> " + strings.ReplaceAll(topic, "\n", "\n> ") + "\n"
	}

	cid := todayCh.GetId()
	subs := getSubscribersNumStr(cid, userMap)
	msgs, talkers := getMsgsAndTalkersNumStr(cid, userMap)
	pins := getPinsNumStr(cid)
	msg := fmt.Sprintf(
		`きなのがチャンネルを紹介するやんね！
## %s
%s
|説明|数|
|:-|:-|
|メンバー数|%s|
|総メッセージ数|%s|
|会話に参加したユーザー数|%s|
|ピン止め数|%s|

直近のメッセージ

https://q.trap.jp/messages/%s
https://q.trap.jp/messages/%s
https://q.trap.jp/messages/%s`,
		fullPath,
		topic,
		subs,
		msgs,
		talkers,
		pins,
		latestMsgs[0].GetId(),
		latestMsgs[1].GetId(),
		latestMsgs[2].GetId(),
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

func getUsersMap() map[string]traq.User {
	users, res, err := cli.UserApi.
		GetUsers(auth).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	userMap := make(map[string]traq.User)
	for _, user := range users {
		userMap[user.GetId()] = user
	}

	return userMap
}

func getLatestMsgs(ch traq.Channel) []traq.Message {
	msgs, res, err := cli.ChannelApi.
		GetMessages(auth, ch.GetId()).
		Limit(3).
		Order("desc").
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	return msgs
}

func getFullPath(ch traq.Channel, chMap map[string]traq.Channel) string {
	fullPath := ch.GetName()
	_ch := ch

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

	return fullPath
}

func getSubscribersNumStr(id string, userMap map[string]traq.User) string {
	subs, res, err := cli.ChannelApi.
		GetChannelSubscribers(auth, id).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	subscribersSummary := fmt.Sprintf("%d人 ", len(subs))
	for _, sub := range subs {
		user, ok := userMap[sub]
		if !ok {
			continue
		}

		subscribersSummary += fmt.Sprintf(":@%s:", user.Name)
	}

	return subscribersSummary
}

func getMsgsAndTalkersNumStr(id string, userMap map[string]traq.User) (string, string) {
	stats, res, err := cli.ChannelApi.
		GetChannelStats(auth, id).
		Execute()
	if err != nil {
		panic(err)
	} else if res.StatusCode != http.StatusOK {
		panic(res.Status)
	}

	msgsCount := fmt.Sprintf("%d件", stats.TotalMessageCount)

	talkersSummary := fmt.Sprintf("%d人 ", len(stats.Users))
	for _, statsUser := range stats.Users {
		user, ok := userMap[statsUser.Id]
		if !ok {
			continue
		}

		talkersSummary += ":@" + user.Name + ":"
	}

	return msgsCount, talkersSummary
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

	return fmt.Sprintf("%d件", len(pins))
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
	} else if res.StatusCode != http.StatusCreated {
		panic(res.Status)
	}
}
