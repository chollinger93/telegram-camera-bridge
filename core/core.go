package core

import (
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.uber.org/zap"
)

type CamModule interface {
	Capture() (image.Image, error)
	Send(image.Image) error
}

type Snapshots struct {
	Cfg    *Config
	Client *http.Client
	TgBot  *tgbotapi.BotAPI
}

func (a *Snapshots) Capture() (image.Image, error) {
	method := "GET"
	req, err := http.NewRequest(method, a.Cfg.Snapshots.SnapshotUrl, nil)
	zap.S().Infof("Sending request to %s", a.Cfg.Snapshots.SnapshotUrl)

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

func (a *Snapshots) Send(img image.Image) error {
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
	a.sendToChat(file)

	return nil
}

func (a *Snapshots) sendToChat(file *os.File) {
	defer os.Remove(file.Name())

	zap.S().Debugf("Authorized on account %s", a.TgBot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	zap.S().Infof("File; %s", file.Name())
	msg := tgbotapi.NewPhotoUpload(a.Cfg.Telegram.ChatId, file.Name())

	_, err := a.TgBot.Send(msg)
	if err != nil {
		zap.S().Error(err)
	}
}
