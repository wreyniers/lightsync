package govee

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    Version
		wantErr error
	}{
		{
			name:    "valid version",
			input:   []byte("1.00.10"),
			want:    Version{Major: 1, Minor: 0, Patch: 10},
			wantErr: nil,
		},
		{
			name:    "invalid version format",
			input:   []byte("1.2"),
			want:    Version{},
			wantErr: ErrInvalidVersionFormat,
		},
		{
			name:    "invalid major version",
			input:   []byte("a.2.3"),
			want:    Version{},
			wantErr: ErrInvalidVersionFormat,
		},
		{
			name:    "invalid minor version",
			input:   []byte("1.b.3"),
			want:    Version{},
			wantErr: ErrInvalidVersionFormat,
		},
		{
			name:    "invalid patch version",
			input:   []byte("1.2.c"),
			want:    Version{},
			wantErr: ErrInvalidVersionFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Version
			err := got.UnmarshalJSON(tt.input)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   Version
		want    []byte
		wantErr error
	}{
		{
			name:    "valid version",
			input:   Version{Major: 1, Minor: 2, Patch: 3},
			want:    []byte(`"1.2.3"`),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.MarshalJSON()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.True(t, bytes.Equal(got, tt.want), "MarshalJSON() got = %v, want %v", got, tt.want)
		})
	}
}

func TestState(t *testing.T) {
	// Test String method
	tests := []struct {
		name  string
		input State
		want  string
	}{
		{"On state", 1, "On"},
		{"Off state", 0, "Off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestColor(t *testing.T) {
	tests := []struct {
		name  string
		input Color
		want  string
	}{
		{"Red color", Color{R: 255, G: 0, B: 0}, "rgb(255, 0, 0)"},
		{"Green color", Color{R: 0, G: 255, B: 0}, "rgb(0, 255, 0)"},
		{"Blue color", Color{R: 0, G: 0, B: 255}, "rgb(0, 0, 255)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
