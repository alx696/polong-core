package remote_control

// 信息json
var InfoJson *string

type VideoInfo struct {
	PresentationTimeUs int64  `json:"presentation_time_us"`
	Data               []byte `json:"data"`
}

// 视频数据信道
var DataChan = make(chan VideoInfo)
