package entity

import (
	"testing"
	"time"
)

func TestBlock_NextBlock(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	tests := []struct {
		name       string
		blockStart time.Time
		now        time.Time
		wantStart  time.Time
		wantEnd    time.Time
		shouldStay bool // true if block should remain the same
	}{
		{
			name:       "time still within current block",
			blockStart: time.Date(2025, 1, 1, 10, 0, 0, 0, loc),  // 10am block
			now:        time.Date(2025, 1, 1, 12, 30, 0, 0, loc), // 12:30pm (within block)
			wantStart:  time.Date(2025, 1, 1, 10, 0, 0, 0, loc),  // same block
			wantEnd:    time.Date(2025, 1, 1, 15, 0, 0, 0, loc),
			shouldStay: true,
		},
		{
			name:       "time exactly at block end",
			blockStart: time.Date(2025, 1, 1, 10, 0, 0, 0, loc), // 10am block
			now:        time.Date(2025, 1, 1, 15, 0, 0, 0, loc), // 3pm (exactly at end)
			wantStart:  time.Date(2025, 1, 1, 15, 0, 0, 0, loc), // next block
			wantEnd:    time.Date(2025, 1, 1, 20, 0, 0, 0, loc),
			shouldStay: false,
		},
		{
			name:       "time past current block - advance one block",
			blockStart: time.Date(2025, 1, 1, 10, 0, 0, 0, loc),  // 10am block
			now:        time.Date(2025, 1, 1, 16, 30, 0, 0, loc), // 4:30pm (in next block)
			wantStart:  time.Date(2025, 1, 1, 15, 0, 0, 0, loc),  // next block
			wantEnd:    time.Date(2025, 1, 1, 20, 0, 0, 0, loc),
			shouldStay: false,
		},
		{
			name:       "time far past - advance multiple blocks",
			blockStart: time.Date(2025, 1, 1, 10, 0, 0, 0, loc), // 10am block
			now:        time.Date(2025, 1, 2, 8, 0, 0, 0, loc),  // next day 8am (4 blocks later)
			wantStart:  time.Date(2025, 1, 2, 6, 0, 0, 0, loc),  // 6am next day (block 4)
			wantEnd:    time.Date(2025, 1, 2, 11, 0, 0, 0, loc), // 11am next day
			shouldStay: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalBlock := NewBlock(tt.blockStart.UTC())
			nextBlock := originalBlock.NextBlock(tt.now)

			if tt.shouldStay {
				// Block should remain the same
				if !nextBlock.StartAt().Equal(originalBlock.StartAt()) {
					t.Errorf("NextBlock() should stay same, got start = %v, want %v",
						nextBlock.StartAt(), originalBlock.StartAt())
				}
			}

			if !nextBlock.StartAt().Equal(tt.wantStart) {
				t.Errorf("NextBlock() start = %v, want %v", nextBlock.StartAt(), tt.wantStart)
			}
			if !nextBlock.EndAt().Equal(tt.wantEnd) {
				t.Errorf("NextBlock() end = %v, want %v", nextBlock.EndAt(), tt.wantEnd)
			}
		})
	}
}

func TestBlock_FormatBlockTime(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	tests := []struct {
		name       string
		blockStart time.Time
		want       string
	}{
		{
			name:       "morning block",
			blockStart: time.Date(2025, 1, 1, 10, 0, 0, 0, loc), // 10am start
			want:       "10am - 3pm",
		},
		{
			name:       "afternoon block",
			blockStart: time.Date(2025, 1, 1, 15, 0, 0, 0, loc), // 3pm start
			want:       "3pm - 8pm",
		},
		{
			name:       "late night block",
			blockStart: time.Date(2024, 12, 31, 23, 0, 0, 0, loc), // 11pm start
			want:       "11pm - 4am",
		},
		{
			name:       "midnight start",
			blockStart: time.Date(2025, 1, 1, 0, 0, 0, 0, loc), // 12am start
			want:       "12am - 5am",
		},
		{
			name:       "noon crossing",
			blockStart: time.Date(2025, 1, 1, 10, 0, 0, 0, loc), // 10am start
			want:       "10am - 3pm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := NewBlock(tt.blockStart.UTC())
			got := block.FormatBlockTime(loc)
			if got != tt.want {
				t.Errorf("FormatBlockTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlock_ValueObjectBehavior(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	t.Run("Block represents specific time period", func(t *testing.T) {
		start := time.Date(2025, 1, 1, 10, 0, 0, 0, loc)
		block := NewBlock(start.UTC())

		// Test getter methods
		if !block.StartAt().Equal(start.UTC()) {
			t.Errorf("StartAt() = %v, want %v", block.StartAt(), start.UTC())
		}

		expectedEnd := start.Add(TimeBlockDuration)
		if !block.EndAt().Equal(expectedEnd.UTC()) {
			t.Errorf("EndAt() = %v, want %v", block.EndAt(), expectedEnd.UTC())
		}

		// Test Period method
		period := block.Period()
		if !period.StartAt().Equal(start.UTC()) {
			t.Errorf("Period().StartAt() = %v, want %v", period.StartAt(), start.UTC())
		}
		if !period.EndAt().Equal(expectedEnd.UTC()) {
			t.Errorf("Period().EndAt() = %v, want %v", period.EndAt(), expectedEnd.UTC())
		}
	})

	t.Run("Block formatting works correctly", func(t *testing.T) {
		// Test the key scenario: 9:46am with 10am start should show "10am - 3pm"
		start := time.Date(2025, 1, 1, 10, 0, 0, 0, loc) // 10am start
		block := NewBlock(start.UTC())

		formatted := block.FormatBlockTime(loc)
		expected := "10am - 3pm"

		if formatted != expected {
			t.Errorf("FormatBlockTime() = %v, want %v", formatted, expected)
		}
	})
}
