package main

import (
	"context"
	"image"
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
	TopCenter = iota
	TopLeft
	TopRight
)

type Time struct {
	Name  	   	string
	PubSubUrl  	string
	KiGoName	string
	Format 		string
	Position 	int
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
}


func main() {
	start := time.Now()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := &Time{}
	err := configuration.ByEnv(cfg)
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
		Changes: []string{"Format"},
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

	configUI := &kc.UIConfig{
		PubSubKiGoUI: valueStartUp.MessageTo.Render,
		PubSubUrl: cfg.PubSubUrl,
		ID: valueStartUp.ID,
	}

	valueUI, valueScreen := kc.GetUIInformation(ctx, configUI)

	if valueUI == nil || valueScreen == nil {
		return
	}
	log.Ctx(ctx).Info("%#v", valueScreen)
	// ###

	objID := 0

	for {
		select {
		case <- ctx.Done():
			if objID == 0 {
				return
			}
			cleanUp(context.Background(), valueStartUp.MessageTo.Render, 
				cfg.PubSubUrl, valueStartUp.ID, valueUI.Channels[0], valueUI.Formats[0], objID)
			return
		default:
		}

		img := CreateSimple(time.Now(), cfg.Format)
		
		configRender := &kc.RenderConfig{
			PubSubKiGoUI: valueStartUp.MessageTo.Render,
			PubSubUrl: cfg.PubSubUrl,
			ID: valueStartUp.ID,
			Channel: valueUI.Channels[0],
			Format: valueUI.Formats[0],
			FPS: 1,
			MaxFrameSize: len(img.Pix),
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

		channel, err := ringbuffer.OpenRingBuffer(valueRender.ChannelName)
		if err != nil {
			log.Ctx(ctx).Err(err)
			if channel != nil {
				channel.Close()
			}
			return
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

		data := util.FromRGBA(uint32(objID), uint16(positionX), uint16(positionY), img)

		_, err = channel.WriteMsg(data)
		if err != nil{
			log.Ctx(ctx).Err(err)
		}
		channel.Close()
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

func cleanUp(ctx context.Context, to string, url string, id string, channel string, format string, objID int) {
	configRender := &kc.RenderConfig{
			PubSubKiGoUI: to,
			PubSubUrl: url,
			ID: id,
			Channel: channel,
			Format: format,
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

	channelCleanUP, err := ringbuffer.OpenRingBuffer(valueRender.ChannelName)
	if err != nil {
		log.Ctx(ctx).Err(err)
		if channelCleanUP != nil {
			channelCleanUP.Close()
		}
		return
	}
	// send empty frame to remove the object
	data := util.FromBytes(objID, 0, 0, 0, 0, 0, []byte{})

	_, err = channelCleanUP.WriteMsg(data)
	if err != nil{
		log.Ctx(ctx).Err(err)
	}
	channelCleanUP.Close()
}