package store

// Defaults and bounds for periodic light refresh (background re-discovery).
const (
	DefaultPollIntervalMs = 1000

	DefaultLightRefreshFirstDelayMinutes  = 10
	DefaultLightRefreshIntervalMinutes    = 15
	DefaultLightRefreshTimeoutSeconds     = 45
	MinLightRefreshFirstDelayMinutes      = 1
	MaxLightRefreshFirstDelayMinutes      = 180
	MinLightRefreshIntervalMinutes        = 1
	MaxLightRefreshIntervalMinutes        = 180
	MinLightRefreshTimeoutSeconds         = 10
	MaxLightRefreshTimeoutSeconds         = 300
)

// NormalizeSettings fills zero/invalid persisted values and clamps to bounds.
func NormalizeSettings(st *Settings) {
	if st.PollIntervalMs <= 0 {
		st.PollIntervalMs = DefaultPollIntervalMs
	}
	if st.LightRefreshFirstDelayMinutes <= 0 {
		st.LightRefreshFirstDelayMinutes = DefaultLightRefreshFirstDelayMinutes
	}
	if st.LightRefreshFirstDelayMinutes < MinLightRefreshFirstDelayMinutes {
		st.LightRefreshFirstDelayMinutes = MinLightRefreshFirstDelayMinutes
	}
	if st.LightRefreshFirstDelayMinutes > MaxLightRefreshFirstDelayMinutes {
		st.LightRefreshFirstDelayMinutes = MaxLightRefreshFirstDelayMinutes
	}
	if st.LightRefreshIntervalMinutes <= 0 {
		st.LightRefreshIntervalMinutes = DefaultLightRefreshIntervalMinutes
	}
	if st.LightRefreshIntervalMinutes < MinLightRefreshIntervalMinutes {
		st.LightRefreshIntervalMinutes = MinLightRefreshIntervalMinutes
	}
	if st.LightRefreshIntervalMinutes > MaxLightRefreshIntervalMinutes {
		st.LightRefreshIntervalMinutes = MaxLightRefreshIntervalMinutes
	}
	if st.LightRefreshTimeoutSeconds <= 0 {
		st.LightRefreshTimeoutSeconds = DefaultLightRefreshTimeoutSeconds
	}
	if st.LightRefreshTimeoutSeconds < MinLightRefreshTimeoutSeconds {
		st.LightRefreshTimeoutSeconds = MinLightRefreshTimeoutSeconds
	}
	if st.LightRefreshTimeoutSeconds > MaxLightRefreshTimeoutSeconds {
		st.LightRefreshTimeoutSeconds = MaxLightRefreshTimeoutSeconds
	}
}
