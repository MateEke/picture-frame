package state

import "time"

// Payload is a sealed interface. Only types defined in this package satisfy it,
// keeping the set of bus event shapes explicit and compiler-checked at publish sites.
type Payload interface{ busPayload() }

type SensorPayload struct {
	DeviceID  string    `json:"device_id"`
	Role      string    `json:"role,omitempty"` // display role, e.g. "inside"; set by the kiosk config mapping
	Kind      string    `json:"kind"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

func (SensorPayload) busPayload() {}

// ScreenPayload carries two distinct facets so consumers don't conflate them:
// On is the live panel power (moves on idle-blank, motion, drift); Auto is the
// manual intent (motion auto-wake enabled), which only a manual on/off changes.
type ScreenPayload struct {
	On   bool `json:"on"`
	Auto bool `json:"auto"`
}

func (ScreenPayload) busPayload() {}

// WeatherPayload carries the latest weather; a zero value (empty IconCode)
// signals a failed poll.
type WeatherPayload struct {
	IconCode string  `json:"icon_code"`
	Temp     float64 `json:"temp"`
	Humidity float64 `json:"humidity"`
}

func (WeatherPayload) busPayload() {}

// ImagePayload carries the current slide's image names (one solo, two for a
// split pair). Empty signals no image.
type ImagePayload struct {
	Names []string `json:"names"`
}

func (ImagePayload) busPayload() {}

// ScreenAspectPayload carries the kiosk's reported aspect (width/height) so the
// admin dashboard can size its preview to match the frame.
type ScreenAspectPayload struct {
	Aspect float64 `json:"aspect"`
}

func (ScreenAspectPayload) busPayload() {}

// TouchPayload carries the moment the on-device kiosk was last tapped.
type TouchPayload struct {
	At time.Time `json:"at"`
}

func (TouchPayload) busPayload() {}

// LibraryPayload signals that the photo or file library changed (upload,
// delete, reorder, slideshow selection) so open views refetch their lists.
type LibraryPayload struct {
	Changed time.Time `json:"changed"`
}

func (LibraryPayload) busPayload() {}

// KioskPayload carries the kiosk overlay's render inputs.
type KioskPayload struct {
	Version       string      `json:"version"` // running build; the kiosk reloads when it changes (post-update)
	Locale        string      `json:"locale"`
	HideClockDate bool        `json:"hide_clock_date"`
	Timezone      string      `json:"timezone"`
	Sensors       []string    `json:"sensors"`
	Weather       bool        `json:"weather"`
	Labels        KioskLabels `json:"labels"`
	Sleep         KioskSleep  `json:"sleep"`
}

// KioskSleep is what the touch UI needs from config.SleepConfig: when to hand
// over to the slideshow, and the night window to show on the home screen.
type KioskSleep struct {
	IdleAfterSeconds int    `json:"idle_after_seconds"`
	Schedule         bool   `json:"schedule"`
	OffFrom          string `json:"off_from"`
	OffUntil         string `json:"off_until"`
}

// KioskLabels mirrors config.KioskLabelsConfig; empty strings hide the caption.
type KioskLabels struct {
	Outside  string `json:"outside"`
	Inside   string `json:"inside"`
	Humidity string `json:"humidity"`
}

func (KioskPayload) busPayload() {}
