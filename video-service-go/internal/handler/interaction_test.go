package handler

import "testing"

func TestParseIDs(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []uint64
		wantErr bool
	}{
		{name: "空串", raw: "", want: nil},
		{name: "正常", raw: "1,2,3", want: []uint64{1, 2, 3}},
		{name: "去重保序", raw: "2, 1, 2 ,3", want: []uint64{2, 1, 3}},
		{name: "非法字符", raw: "1,abc", wantErr: true},
		{name: "零值", raw: "0", wantErr: true},
		{name: "负号", raw: "-1", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIDs(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际返回 %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望报错: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ids = %v, 期望 %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("ids = %v, 期望 %v", got, tc.want)
				}
			}
		})
	}
}
