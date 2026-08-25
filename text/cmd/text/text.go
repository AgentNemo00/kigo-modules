package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"time"

	kc "github.com/AgentNemo00/kigo-code"
	"github.com/AgentNemo00/kigo-core/order"
	"github.com/AgentNemo00/kigo-core/util"
	"github.com/AgentNemo00/sca-instruments/configuration"
	"github.com/AgentNemo00/sca-instruments/containerization"
	"github.com/AgentNemo00/sca-instruments/log"
	"github.com/AgentNemo00/sca-instruments/pubsub/nats"
	"github.com/gogpu/ui/offscreen"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
)

const(
	Format = "PNG"
	Channel = "PubSub"
)

type Text struct {
	Name  	   	string
	PubSubUrl  	string
	KiGoName	string
	Value 		string
}

func (t *Text) Default() {
	if t.Name == "" {
		t.Name = "text"
	}
	if t.PubSubUrl == "" {
		t.PubSubUrl = "nats://127.0.0.1:4222"
	}
	if t.KiGoName == "" {
		t.KiGoName = "KiGo"
	}
	if t.Value == "" {
		t.Value = "Welcome to KiGo"
	}
}


func main() {
	start := time.Now()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := &Text{}
	err := configuration.ByEnvWithPrefix("TEXT", cfg)
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
		Changes: []string{"Value"},
		Heartbeat: time.Hour*24,
	}

	valueStartUp := kc.InitializeModule(ctx, start, configInit, func(payload order.OrderShutdownPayload) {
		log.Ctx(ctx).Warn(payload.Reason)
		cancel()
	})
	
	if valueStartUp == nil {
		return
	}

	//###

	configChances := &kc.ChangesConfig{
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
		Changes: configInit.Changes,
	}

	cancelSub, err := kc.ListenForChanges(ctx, configChances, func (change string, value any)  {
		switch(change) {
			case "Value":
				str, ok := value.(string)
				if !ok {
					log.Ctx(ctx).Error("invalid value for value change")
					return
				}
				cfg.Value = str
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

	configUI := &kc.UIConfig{
		PubSubKiGoUI: valueStartUp.MessageTo.Render,
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
	}

	valueUI, valueScreen := kc.GetUIInformation(ctx, configUI)
	
	log.Ctx(ctx).Info("%#v", valueUI)

	if valueUI == nil || valueScreen == nil {
		return
	}
	log.Ctx(ctx).Info("%#v", valueScreen)
	// ###

	objID := 0
	i := 0
	for {
		select {
		case <- ctx.Done():
			if objID == 0 {
				return
			}
			log.Ctx(ctx).Info("cleanup")
			cleanUp(context.Background(), cfg.PubSubUrl, valueStartUp.MessageTo.Render, 
				cfg.PubSubUrl, valueStartUp.ID, Channel, objID)
			return
		default:
		}

		img := CreateSimple(fmt.Sprintf("%s %d",cfg.Value, i))
		width := img.Rect.Bounds().Dx()
		height := img.Rect.Bounds().Dy()


		var buf bytes.Buffer
		encoder := png.Encoder{
			CompressionLevel: png.BestCompression,
		}
		err := encoder.Encode(&buf, img)
		if err != nil {
			log.Ctx(ctx).Err(err)
		}

		imgRaw := buf.Bytes()
		dataLength := len(imgRaw)

		configRender := &kc.RenderConfig{
			PubSubKiGoUI: valueStartUp.MessageTo.Render,
			PubSubUrl: cfg.PubSubUrl,
			UUID: valueStartUp.ID,
			Channel: Channel,
			Format: Format,
			FPS: 1,
			MaxFrameSize: dataLength,
			ObjectID: objID,
			Timeout: time.Second,
			Time: 0,
		}

		valueRender := kc.GetChannel(ctx, configRender)

		if valueRender == nil {
			return
		}

		if objID == 0 {
			objID = valueRender.ObjectID
		}

		positionX := (valueRender.ScreenWidth / 2) - (img.Rect.Dx() / 2)
		positionY := (valueRender.ScreenHeight / 2) - (img.Rect.Dy() / 2)


		data := util.FromBytesSigned(uint32(objID), uint16(positionX), uint16(positionY), uint16(width), uint16(height), uint32(dataLength), imgRaw)

		log.Ctx(ctx).Info("length %d", len(imgRaw))

		pub, err := nats.PublisherWithURL[[]byte](cfg.PubSubUrl)
		if err != nil {
			log.Ctx(ctx).Err(err)
			continue
		}
		err = pub.Publish(ctx, valueRender.ChannelName, data)
		if err != nil {
			log.Ctx(ctx).Err(err)
		}
		i++
		time.Sleep(time.Second)
	
	}
}

func CreateSimple(value string) *image.RGBA {
	m3 := material3.New(widget.Hex(0x6750A4))
	r := offscreen.NewRenderer(0, 0,
		offscreen.WithFitSize(),
		offscreen.WithTheme(m3),
	)
	label := primitives.Box(
		primitives.Text(value).
			FontSize(32).
			Bold().
			Color(widget.RGBA8(225, 225, 255, 255)),
	).Padding(10)
	r.Render(label)
	return r.Image()
}

func cleanUp(ctx context.Context, pubsuburl, to string, url string, id string, channel string, objID int) {
	configRender := &kc.RenderConfig{
			PubSubKiGoUI: to,
			PubSubUrl: url,
			UUID: id,
			Channel: channel,
			Format: "RAW",
			FPS: 1,
			MaxFrameSize: 1,
			ObjectID: objID,
			Timeout: time.Second,
			Time: 0,
	}
	valueRender := kc.GetChannel(ctx, configRender)

	if valueRender == nil {
		return
	}

	// send empty frame to remove the object
	data := util.FromBytes(objID, 0, 0, 0, 0, 0, []byte{})

	pub, err := nats.PublisherWithURL[[]byte](pubsuburl)
	if err != nil {
		log.Ctx(ctx).Err(err)
		
	}
	err = pub.Publish(ctx, valueRender.ChannelName, data)
	if err != nil {
		log.Ctx(ctx).Err(err)
	}
}