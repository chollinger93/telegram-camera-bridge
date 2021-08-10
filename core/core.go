package core

import (
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.uber.org/zap"
)

type CamModule interface {
	Capture() (image.Image, error)
	Send(image.Image) error
	SendTo(image.Image, int) error
	HandleCommands(chan int) error
}

type Snapshots struct {
	Cfg    *Config
	Client *http.Client
	TgBot  *tgbotapi.BotAPI
}

func (a *Snapshots) Capture() (image.Image, error) {
	method := "GET"
	req, err := http.NewRequest(method, a.Cfg.Snapshots.SnapshotUrl, nil)
	zap.S().Debugf("Sending request to %s", a.Cfg.Snapshots.SnapshotUrl)

	if err != nil {
		return nil, err
	}
	req.Header.Add("Cookie", "capture_fps_1=14.0; monitor_info_1=; motion_detected_1=false")

	res, err := a.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	img, _, err := image.Decode(res.Body)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (a *Snapshots) SendTo(img image.Image, replyId int) error {
	// Temp
	file, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	// Save
	// TODO: in-memory
	out, _ := os.Create(file.Name())
	defer out.Close()

	err = jpeg.Encode(out, img, nil)
	if err != nil {
		return err
	}

	// Telegram
	go a.sendToChat(file, replyId)

	return nil
}

func (a *Snapshots) Send(img image.Image) error {
	return a.SendTo(img, 0)
}

func (a *Snapshots) sendToChat(file *os.File, replyId int) {
	defer os.Remove(file.Name())

	zap.S().Debugf("Authorized on account %s", a.TgBot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	zap.S().Infof("File; %s", file.Name())
	msg := tgbotapi.NewPhotoUpload(a.Cfg.Telegram.ChatId, file.Name())
	if replyId != 0 {
		msg.ReplyToMessageID = replyId
	}
	_, err := a.TgBot.Send(msg)
	if err != nil {
		zap.S().Error(err)
	}
}

func (a *Snapshots) HandleCommands(ch chan int) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := a.TgBot.GetUpdatesChan(u)

	if err != nil {
		return err
	}

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		zap.S().Debugf("[%s] `%s`", update.Message.From.UserName, update.Message.Text)

		// Take a snapshot
		if update.Message.Text == "/snap" || update.Message.Text == fmt.Sprintf("%s %s", a.Cfg.Telegram.BotName, "/snap") {
			img, err := a.Capture()
			if err != nil {
				zap.S().Error(err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Can't do that right now")
				msg.ReplyToMessageID = update.Message.MessageID
				a.TgBot.Send(msg)

				return err
			}
			// Send picture ootherwise
			a.SendTo(img, update.Message.MessageID)
		} else if strings.HasPrefix(update.Message.Text, "/interval") {
			elems := strings.Split(update.Message.Text, " ")
			if len(elems) != 2 {
				err := fmt.Errorf("Usage: `/interval 60` to set the snapshot interval to every 10mins")
				zap.S().Error(err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
				msg.ReplyToMessageID = update.Message.MessageID
				a.TgBot.Send(msg)
				return err
			}
			// Convert
			i, err := strconv.Atoi(elems[1])
			if err != nil {
				err := fmt.Errorf("%s is not a valid number of seconds", elems[1])
				zap.S().Error(err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
				msg.ReplyToMessageID = update.Message.MessageID
				a.TgBot.Send(msg)
				return err
			}
			// Overwrite interval
			a.Cfg.Snapshots.IntervalS = i
			// Inform ticker to reset
			ch <- i
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Will now take snapshots every %v seconds (takes time to take effect)", i))
			msg.ReplyToMessageID = update.Message.MessageID
			a.TgBot.Send(msg)
		} else if strings.HasPrefix(update.Message.Text, "/help") {
			tmsg := `Usage:
			/snap - take a snapshot
			/interval <seconds> - set the interval for snapshots
			`
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, tmsg)
			msg.ReplyToMessageID = update.Message.MessageID
			a.TgBot.Send(msg)
		} else {
			err := fmt.Sprintf("Unknown command `%s` from %s", update.Message.Text, update.Message.From)
			zap.S().Errorf(err)
		}
	}
	return nil
}
