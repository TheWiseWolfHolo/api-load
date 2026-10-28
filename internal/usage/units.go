package usage

// Units 保存上游明确报告的量，nil 表示未报告。各字段不能互相换算。
type Units struct {
	Images            *int64 `json:"images,omitempty,string"`
	ImageTokens       *int64 `json:"image_tokens,omitempty,string"`
	AudioMilliseconds *int64 `json:"audio_milliseconds,omitempty,string"`
	Pages             *int64 `json:"pages,omitempty,string"`
	SearchUnits       *int64 `json:"search_units,omitempty,string"`
}

func (u Units) HasPositive() bool {
	for _, v := range []*int64{u.Images, u.ImageTokens, u.AudioMilliseconds, u.Pages, u.SearchUnits} {
		if v != nil && *v > 0 {
			return true
		}
	}
	return false
}

// Clone detaches quantities before an observation crosses an asynchronous boundary.
func (u Units) Clone() Units {
	clone := u
	for _, target := range []**int64{&clone.Images, &clone.ImageTokens, &clone.AudioMilliseconds, &clone.Pages, &clone.SearchUnits} {
		if *target != nil {
			value := **target
			*target = &value
		}
	}
	return clone
}
