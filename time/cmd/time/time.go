package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"slices"
	"time"

	kc "github.com/AgentNemo00/kigo-code"
	"github.com/AgentNemo00/kigo-core/order"
	"github.com/AgentNemo00/kigo-core/util"
	"github.com/AgentNemo00/sca-instruments/configuration"
	"github.com/AgentNemo00/sca-instruments/containerization"
	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/EBWi11/mmap_ringbuffer"
	"github.com/gogpu/ui/offscreen"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
)

const(
	TopLeft = iota +1
	TopCenter
	TopRight
)

const (
	RAW = "RAW"
	PNG = "PNG"
	JPEG = "JPEG"
)

type Time struct {
	Name  	   	string
	PubSubUrl  	string
	KiGoName	string
	Format 		string
	Position 	int
	Encoding 	string
}

func (t *Time) Default() {
	if t.Name == "" {
		t.Name = "clock"
	}
	if t.PubSubUrl == "" {
		t.PubSubUrl = "nats://127.0.0.1:4222"
	}
	if t.KiGoName == "" {
		t.KiGoName = "KiGo"
	}
	if t.Format == "" {
		t.Format = "15:04:05"
	}
	if t.Encoding == ""  {
		t.Encoding = PNG
	}
	if t.Position == 0 {
		t.Position = TopLeft
	}	
}


func main() {
	// Initialize module

	// track start time
	start := time.Now()

	ctx, cancel := context.WithCancel(context.Background())

	// Get config
	cfg := &Time{}
	err := configuration.ByEnvWithPrefix("TIME", cfg)
	if err != nil {
		log.Ctx(ctx).Err(err)
		return
	}
	

	containerization.Callback(func ()  {
		cancel()
	})

	go containerization.Interrupt(func() {})

	configInit := &kc.InitConfig{
		Name: cfg.Name,
		PubSubKiGo: cfg.KiGoName,
		PubSubUrl: cfg.PubSubUrl,
		Changes: []string{"Format", "Position"},
		Heartbeat: time.Hour*24,
	}

	// First call to KIGO for initialization
	valueStartUp := kc.InitializeModule(ctx, start, configInit, func(payload order.OrderShutdownPayload) {
		log.Ctx(ctx).Warn("%s", payload.Reason)
		cancel()
	})
	if valueStartUp == nil {
		return
	}

	

	// create callback for changes trigger by other modules
	configChances := &kc.ChangesConfig{
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
		Changes: configInit.Changes,
	}
	cancelSub, err := kc.ListenForChanges(ctx, configChances, func (change string, value any)  {
		switch(change) {
			case "Format":
				str, ok := value.(string)
				if !ok {
					log.Ctx(ctx).Error("invalid value for format change")
					return
				}
				cfg.Format = str
			case "Position":
				pos, ok := value.(int)
				if !ok {
					log.Ctx(ctx).Error("invalid value for position change")
					return
				}
				cfg.Position = pos
			default:
				log.Ctx(ctx).Warn("unknown change: %s", change)
		}
	})

	containerization.Callback(func ()  {
		cancelSub()
	})

	if err != nil {
		log.Ctx(ctx).Err(err)
		return
	}

	// create config for UI information
	configUI := &kc.UIConfig{
		PubSubKiGoUI: valueStartUp.MessageTo.Render,
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
	}

	// get UI informations
	valueUI, valueScreen := kc.GetUIInformation(ctx, configUI)
	if valueUI == nil || valueScreen == nil {
		return
	}



	format := RAW

	// check for encoding format, if not supported by KiGo, fallback to RAW
	if slices.Contains(valueUI.Formats, cfg.Encoding) {
		format = cfg.Encoding
	}

	img := CreateSimple(time.Now(), cfg.Format)

	objID := 0

		// create render desire
	configRender := &kc.RenderConfig{
		PubSubKiGoUI: valueStartUp.MessageTo.Render, // KIGOUI sub
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,						// module ID
		Channel: valueUI.Channels[0],				// channel to use
		Format: format,								// format to use
		FPS: 1,										// FPS per second
		MaxFrameSize: len(img.Pix), 					// max size of frame to send
		ObjectID: objID,							// object ID
		Timeout: time.Second,                       // timeout per frame
		Time: 0,
	}

	valueRender := kc.GetChannel(ctx, configRender)

	if objID == 0 {
		objID = valueRender.ObjectID
	}

	positionX := 0
	positionY := 0

	switch (cfg.Position) {
		case TopLeft:
			positionX = 0
			positionY = 0
		case TopRight:
			positionX = valueRender.ScreenWidth - img.Rect.Dx()
			positionY = 0
		case TopCenter:
			positionX = (valueRender.ScreenWidth / 2) - (img.Rect.Dx() / 2)
			positionY = 0
	}

	channel, err := ringbuffer.OpenRingBuffer(valueRender.ChannelName)
	if err != nil {
		log.Ctx(ctx).Err(err)
		if channel != nil {
			channel.Close()
		}
		return
	}


	for {
		select {
		case <- ctx.Done():
			if objID == 0 {
				return
			}
			data := util.FromBytes(objID, 0, 0, 0, 0, 0, []byte{})
			_, err = channel.WriteMsg(data)
			if err != nil{
				log.Ctx(ctx).Err(err)
			}
			channel.Close()
			return
		default:
		}

		// create image with current time
		img := CreateSimple(time.Now(), cfg.Format)

		// define image properties
		width := img.Rect.Bounds().Dx()
		height := img.Rect.Bounds().Dy()
		imgRaw := img.Pix
		dataLength := len(imgRaw)

		// encode image to desired format
		switch(cfg.Encoding) {
			case RAW:
			case PNG:
				var buf bytes.Buffer
				encoder := png.Encoder{
					CompressionLevel: png.BestCompression,
				}
				err := encoder.Encode(&buf, img)
				if err != nil {
					log.Ctx(ctx).Err(err)
				}
				imgRaw = buf.Bytes()
				dataLength = len(imgRaw)
			case JPEG:
				var buf bytes.Buffer

				err := jpeg.Encode(&buf, img, &jpeg.Options{
					Quality: 80, // 1–100
				})
				if err != nil {
					log.Ctx(ctx).Err(err)
				}

				imgRaw = buf.Bytes()
				dataLength = len(imgRaw)
			default:
				log.Ctx(ctx).Warn("unknown encoding: %s", cfg.Encoding)
		}

		data := util.FromBytesSigned(uint32(objID), uint16(positionX), uint16(positionY), uint16(width), uint16(height), uint32(dataLength), imgRaw)

		_, err = channel.WriteMsg(data)
		if err != nil{
			log.Ctx(ctx).Err(err)
		}
		time.Sleep(time.Second)
	}
}

func CreateSimple(t time.Time, format string) *image.RGBA {
	m3 := material3.New(widget.Hex(0x6750A4))
	r := offscreen.NewRenderer(0, 0,
		offscreen.WithFitSize(),
		offscreen.WithTheme(m3),
	)
	now := t.Format(format)
	label := primitives.Box(
		primitives.Text(now).
			FontSize(32).
			Bold().
			Color(widget.RGBA8(225, 225, 255, 255)),
	).Padding(10)
	r.Render(label)
	return r.Image()
}
