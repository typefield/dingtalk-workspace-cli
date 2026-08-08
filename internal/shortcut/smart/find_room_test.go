package smart

import "testing"

func TestFindRoomParseMillisAndValidation(t *testing.T) {
	start, err := findRoomParseMillis("start", "2026-03-10T14:00:00+08:00")
	if err != nil || start <= 0 {
		t.Fatalf("valid start: millis=%d err=%v", start, err)
	}
	if _, err := findRoomParseMillis("start", "not-a-time"); err == nil {
		t.Fatal("invalid timestamp must return typed validation error")
	}
}

func TestFindRoomProjectionNestedShape(t *testing.T) {
	data := map[string]any{
		"result": map[string]any{
			"rooms": []any{
				map[string]any{"roomId": "r1", "roomName": "A", "capacity": float64(8)},
			},
		},
	}
	rooms := findRoomExtractRooms(data)
	if len(rooms) != 1 || rooms[0]["roomId"] != "r1" {
		t.Fatalf("nested room projection = %#v", rooms)
	}
}
