package cmd

import (
	"fmt"
	"testing"
	"time"
)

func Test_isWithinActiveHours(t *testing.T) {
	baseDt := time.Date(1776, 7, 4, 12, 0, 0, 0, time.UTC)
	type args struct {
		now   string
		start string
		end   string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "isWithin",
			args: args{
				now:   fmt.Sprintf("%02d:%02d", baseDt.Hour(), baseDt.Minute()),
				start: "08:00",
				end:   "18:00",
			},
			want: true,
		},
		{
			name: "isOutside",
			args: args{
				now:   fmt.Sprintf("%02d:%02d", baseDt.Hour(), baseDt.Minute()),
				start: "14:00",
				end:   "18:00",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWithinActiveHours(tt.args.now, tt.args.start, tt.args.end); got != tt.want {
				t.Errorf("isWithinActiveHours() = %v, want %v", got, tt.want)
			}
		})
	}
}
