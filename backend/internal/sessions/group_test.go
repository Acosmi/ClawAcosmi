package sessions

import (
	"testing"
)

func TestBuildGroupDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		params GroupDisplayNameParams
		want   string
	}{
		{
			"basic with subject",
			GroupDisplayNameParams{Provider: "telegram", Subject: "My Group", Key: "key1"},
			"telegram:g-my-group",
		},
		{
			"with channel",
			GroupDisplayNameParams{Provider: "slack", GroupChannel: "#general", Key: "key1"},
			"slack:#general",
		},
		{
			"channel + space",
			GroupDisplayNameParams{Provider: "slack", GroupChannel: "#dev", Space: "workspace", Key: "key1"},
			"slack:workspace#dev",
		},
		{
			"no detail, fallback to id",
			GroupDisplayNameParams{Provider: "telegram", ID: "12345", Key: "key1"},
			"telegram:g-12345",
		},
		{
			"empty provider",
			GroupDisplayNameParams{Subject: "Test", Key: "key1"},
			"group:g-test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildGroupDisplayName(tt.params)
			if got != tt.want {
				t.Errorf("BuildGroupDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveGroupSessionKey(t *testing.T) {
	tests := []struct {
		name string
		ctx  MsgContextForGroup
		want *GroupKeyResolution
	}{
		{
			"whatsapp group",
			MsgContextForGroup{From: "12345@g.us", Provider: "whatsapp"},
			&GroupKeyResolution{Key: "whatsapp:group:12345@g.us", Channel: "whatsapp", ID: "12345@g.us", ChatType: "group"},
		},
		{
			"telegram group with chatType",
			MsgContextForGroup{From: "telegram:group:mygrp", ChatType: "group"},
			&GroupKeyResolution{Key: "telegram:group:mygrp", Channel: "telegram", ID: "mygrp", ChatType: "group"},
		},
		{
			"direct message - not a group",
			MsgContextForGroup{From: "+1234567890", ChatType: ""},
			nil,
		},
		{
			"slack channel",
			MsgContextForGroup{From: "slack:channel:C12345", ChatType: "channel"},
			&GroupKeyResolution{Key: "slack:channel:c12345", Channel: "slack", ID: "c12345", ChatType: "channel"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveGroupSessionKey(tt.ctx)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if got.Key != tt.want.Key {
				t.Errorf("Key = %q, want %q", got.Key, tt.want.Key)
			}
			if got.Channel != tt.want.Channel {
				t.Errorf("Channel = %q, want %q", got.Channel, tt.want.Channel)
			}
			if got.ChatType != tt.want.ChatType {
				t.Errorf("ChatType = %q, want %q", got.ChatType, tt.want.ChatType)
			}
		})
	}
}

func TestNormalizeGroupLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Group Name", "my-group-name"},
		{"  hello   world  ", "hello-world"},
		{"#channel", "#channel"},
		{"", ""},
		{"---test---", "test"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeGroupLabel(tt.input)
			if got != tt.want {
				t.Errorf("normalizeGroupLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShortenGroupID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "short"},
		{"exactly14chrs", "exactly14chrs"},
		{"this-is-a-very-long-group-id", "this-i...p-id"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortenGroupID(tt.input)
			if got != tt.want {
				t.Errorf("shortenGroupID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
