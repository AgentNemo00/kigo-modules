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
	ringbuffer "github.com/EBWi11/mmap_ringbuffer"
	"github.com/gogpu/ui/offscreen"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
)

const(
	Format = "RAW"
	Channel = "IPC"
)

type Text struct {
	Name  	   	string
	PubSubUrl  	string
	KiGoName	string
	Value 		string
}

func (t *Text) Default() {
	if t.Name == "" {
		t.Name = "bounce"
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
		Changes: []string{},
		Heartbeat: time.Minute,
	}
	valueStartUp := kc.InitializeModule(ctx, start, configInit, func(payload order.OrderShutdownPayload) {
		log.Ctx(ctx).Warn(payload.Reason)
		cancel()
	})

	configUI := &kc.UIConfig{
		PubSubKiGoUI: valueStartUp.MessageTo.Render,
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
	}

	_, valueScreen := kc.GetUIInformation(ctx, configUI)
	
	maxHeight := valueScreen.Height
	maxWidth := valueScreen.Width

	objID := 0
	x := 1
	y := 1
	dx, dy := 1, 1

	img := CreateSimple(cfg.Value)

	configRender := &kc.RenderConfig{
		PubSubKiGoUI: valueStartUp.MessageTo.Render,
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
		Channel: Channel,
		Format: Format,
		FPS: 30,
		MaxFrameSize: len(img.Pix),
		ObjectID: objID,
		Timeout: time.Second,
		Time: 0,
	}

	valueRender := kc.GetChannel(ctx, configRender)

	channel, err := ringbuffer.OpenRingBuffer(valueRender.ChannelName)
	if err != nil {
		log.Ctx(ctx).Err(err)
		if channel != nil {
			channel.Close()
		}
		return
	}
	log.Ctx(ctx).Info("Channel created")

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

		img := CreateSimple(cfg.Value)
		width := img.Rect.Bounds().Dx()
		height := img.Rect.Bounds().Dy()
		imgRaw := img.Pix
		dataLength := len(imgRaw)

		if objID == 0 {
			objID = valueRender.ObjectID
		}

		newX := x + dx
		newY := y + dy

		if newX < 1 || newX+img.Rect.Dx() >= maxWidth {
			dx = -dx 
		} else {
			x = newX
		}

		if newY < 1 || newY + img.Rect.Dy() >= maxHeight {
			dy = -dy 
		} else {
			y = newY
		}

		data := util.FromBytesSigned(uint32(objID), uint16(x), uint16(y), uint16(width), uint16(height), uint32(dataLength), imgRaw)

		_, err = channel.WriteMsg(data)
		if err != nil{
			log.Ctx(ctx).Err(err)
		}

		time.Sleep((time.Millisecond * 1000)/30) // Approximately 30 FPS
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
