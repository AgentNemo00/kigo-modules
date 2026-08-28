package main

import (
	"context"
	"image"
	"sync"
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
	SlideBottom = iota + 1
	SlideUp
)

type Text struct {
	Name  	   	string
	PubSubUrl  	string
	KiGoName	string
	Type 		int
	queue		[]string
	m 			*sync.Mutex
}

func (t *Text) Default() {
	if t.Name == "" {
		t.Name = "notification"
	}
	if t.PubSubUrl == "" {
		t.PubSubUrl = "nats://127.0.0.1:4222"
	}
	if t.KiGoName == "" {
		t.KiGoName = "KiGo"
	}
	if t.Type == 0 {
		t.Type = SlideBottom
	}
	t.queue = make([]string, 0)
	t.m = &sync.Mutex{}
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
		Changes: []string{"Notify"},
		Heartbeat: time.Minute,
	}
	valueStartUp := kc.InitializeModule(ctx, start, configInit, func(payload order.OrderShutdownPayload) {
		log.Ctx(ctx).Warn(payload.Reason)
		cancel()
	})


	configChances := &kc.ChangesConfig{
		PubSubUrl: cfg.PubSubUrl,
		UUID: valueStartUp.ID,
		Changes: configInit.Changes,
	}

	cancelSub, err := kc.ListenForChanges(ctx, configChances, func (change string, value any)  {
		switch(change) {
			case "Notify":
				str, ok := value.(string)
				if !ok {
					log.Ctx(ctx).Error("invalid value for value change")
					return
				}
				cfg.queue = append(cfg.queue, str)
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

	_, valueScreen := kc.GetUIInformation(ctx, configUI)
	
	maxHeight := valueScreen.Height
	maxWidth := valueScreen.Width

	time.AfterFunc(time.Second*3, func ()  {
		cfg.queue = append(cfg.queue, "Test")
	})
	
	for {
		select{
		case <- ctx.Done():
			return
		default:
			if len(cfg.queue) == 0 {
				continue
			}

			objID := 0
			img := CreateSimple(cfg.queue[0])
			width := img.Rect.Bounds().Dx()
			height := img.Rect.Bounds().Dy()
			imgRaw := img.Pix
			dataLength := len(imgRaw)


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



			positionX := (maxWidth / 2) - (img.Rect.Dx() / 2)
			startY := maxHeight - img.Rect.Dy()
			endY := maxHeight - (maxHeight/5)
			step := 1
			positionY := startY
			log.Ctx(ctx).Info("Start: %d, end: %d, step: %d,", startY, endY, step)
			animationTimeStart := time.Now()

			animation: for {
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

				data := util.FromBytesSigned(uint32(objID), uint16(positionX), uint16(positionY), uint16(width), uint16(height), uint32(dataLength), imgRaw)

				_, err = channel.WriteMsg(data)
				if err != nil{
					log.Ctx(ctx).Err(err)
				}

				if time.Now().After(animationTimeStart.Add(time.Millisecond*33)) {
					positionY -= step
					animationTimeStart = time.Now()
				}

				if positionY <= endY {
					break animation
				}

				time.Sleep((time.Millisecond * 1000)/30) // Approximately 30 FPS
			}
			
			data := util.FromBytes(objID, 0, 0, 0, 0, 0, []byte{})
			_, err = channel.WriteMsg(data)
			if err != nil{
				log.Ctx(ctx).Err(err)
			}
			channel.Close()


			cfg.m.Lock()
			cfg.queue = cfg.queue[1:]
			cfg.m.Unlock()
		}
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
			Color(widget.RGBA8(0, 0, 0, 255)),
	).Padding(10).Background(widget.RGBA8(225, 225, 255, 255))
	r.Render(label)
	return r.Image()
}
