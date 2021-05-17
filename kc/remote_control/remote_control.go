package remote_control

// 信息json
var InfoJson *string

// 视频数据信道
var DataChan = make(chan []byte)
