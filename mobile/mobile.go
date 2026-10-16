//go:build android

// Package mobile connects the shared Go game to Android's Ebitengine view.
package mobile

import (
	"log"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"krytonegg/internal/presentation"
)

type command struct {
	kind          string
	directory     string
	ticks, combat int
}

// The Android UI thread queues commands; only the game thread mutates the app.
type host struct {
	app                       *presentation.App
	commands                  chan command
	directory                 string
	verifyTicks, verifyCombat int
	pausePending, reported    bool
	atTitle                   atomic.Bool
}

var gameHost = &host{commands: make(chan command, 32)}

func init() {
	ebiten.SetTPS(50)
	ebiten.SetScreenFilterEnabled(false)
	enginemobile.SetGame(gameHost)
}

// Configure must run before EbitenView creation, with Android's private files directory.
func Configure(dataDir string) { gameHost.commands <- command{kind: "configure", directory: dataDir} }

// ConfigureVerification runs deterministic input and holds its final frame for capture.
func ConfigureVerification(ticks, combat int) {
	gameHost.commands <- command{kind: "verify", ticks: ticks, combat: combat}
}

// RequestPause preserves progress and pauses an active game after a lifecycle transition.
func RequestPause() { gameHost.enqueue(command{kind: "pause"}) }

// RequestBack gives Android's Back gesture the same contextual navigation as touch controls.
func RequestBack() { gameHost.enqueue(command{kind: "back"}) }

// IsAtTitle lets the activity finish normally when Back is pressed on the title screen.
func IsAtTitle() bool { return gameHost.atTitle.Load() }

// Dummy retains a stable exported symbol for the Android binding generator.
func Dummy() {}

func (h *host) enqueue(request command) {
	select {
	case h.commands <- request:
	default:
	}
}

func (h *host) Update() error {
	for {
		select {
		case request := <-h.commands:
			switch request.kind {
			case "configure":
				if h.app == nil {
					h.directory = request.directory
				}
			case "verify":
				if h.app == nil {
					h.verifyTicks = max(0, min(request.ticks, 50000))
					h.verifyCombat = max(0, min(request.combat, 6))
				}
			case "pause":
				h.pausePending = true
			case "back":
				if h.app != nil {
					h.app.Back()
				}
			}
		default:
			goto configured
		}
	}
configured:
	if h.app == nil {
		// JNI context and the view must exist before graphics or audio are created.
		if h.directory == "" {
			return nil
		}
		var err error
		h.app, err = presentation.New(presentation.Options{Scale: 4, Seed: 1990, Mobile: true, DataDir: h.directory, SmokeTicks: h.verifyTicks, StartCombat: h.verifyCombat, FreezeCheck: true})
		if err != nil {
			return err
		}
	}
	if h.pausePending {
		h.app.Suspend()
		h.pausePending = false
	}
	if err := h.app.Update(); err != nil {
		return err
	}
	h.atTitle.Store(h.app.AtTitle())
	if h.verifyTicks > 0 && h.app.VerificationDone() && !h.reported {
		log.Printf("krytonegg_android_check %s", h.app.CheckReport())
		h.reported = true
	}
	return nil
}

func (h *host) Draw(screen *ebiten.Image) {
	if h.app != nil {
		h.app.Draw(screen)
	}
}

func (h *host) Layout(width, height int) (int, int) {
	if h.app != nil {
		return h.app.Layout(width, height)
	}
	return max(1, width), max(1, height)
}
