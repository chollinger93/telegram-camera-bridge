package core

import (
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"go.uber.org/zap"
)

type CamModule interface {
	Capture() (image.Image, error)
	Send(image.Image) error
}

type Snapshots struct {
	Cfg    *Config
	Client *http.Client
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
	out, _ := os.Create("./img.jpg")
	defer out.Close()

	//var opts jpeg.Options
	//opts.Quality = 1

	err := jpeg.Encode(out, img, nil)
	if err != nil {
		log.Println(err)
	}
	return nil
	/*
		method := "POST"
		req, err := http.NewRequest(method, a.Cfg.Snapshots.SendUrl, bytesBody(body))

		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Cookie", "capture_fps_1=14.0; monitor_info_1=; motion_detected_1=false")

		res, err := a.Client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		return nil
	*/
}
