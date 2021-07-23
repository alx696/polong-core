package db

import (
	"fmt"
	"os"

	"database/sql"
	// 必须
	_ "github.com/mattn/go-sqlite3"
)

// ChatMessageInfo 会话消息信息
type ChatMessageInfo struct {
	ID                       int64  `json:"id"`
	FromPeerID               string `json:"fromPeerID"`
	ToPeerID                 string `json:"toPeerID"`
	Text                     string `json:"text"`
	FileSize                 int64  `json:"fileSize"`
	FileNameWithoutExtension string `json:"fileNameWithoutExtension"`
	FileExtension            string `json:"fileExtension"`
	FileDirectory            string `json:"fileDirectory"` // 空字符时用默认目录
	State                    string `json:"state"`         // 发送/接收,进度,完成
	Read                     bool   `json:"read"`
}

var db *sql.DB

// Open 打开
func Open(path string) error {
	// 创建数据库
	_, e := os.Stat(path)
	if os.IsNotExist(e) {
		dbFile, e := os.Create(path)
		if e != nil {
			return e
		}
		dbFile.Close()
	}

	// 打开数据库
	db, e = sql.Open("sqlite3", path)
	if e != nil {
		return e
	}

	// 创建数据表
	_, e = db.Exec(`
		CREATE TABLE IF NOT EXISTS "chat_message" (
			"id"	TEXT NOT NULL UNIQUE,
			"fromPeerID"	TEXT NOT NULL,
			"toPeerID"	TEXT NOT NULL,
			"text"	TEXT,
			"fileSize"	INTEGER,
			"fileNameWithoutExtension"	TEXT,
			"fileExtension"	TEXT,
			"fileDirectory" TEXT,
			"state"	TEXT NOT NULL,
			"read" BOOL NOT NULL,
			PRIMARY KEY("id")
		);
	`)
	if e != nil {
		return e
	}

	return nil
}

// Close 关闭
func Close() {
	db.Close()
}

// ChatMessageInfoInsert 插入会话消息
func ChatMessageInfoInsert(m *ChatMessageInfo) error {
	_, e := db.Exec(fmt.Sprintf(`insert into chat_message values(%d, '%s', '%s', '%s', %d, '%s', '%s', '%s', '%s', %v)`,
		m.ID, m.FromPeerID, m.ToPeerID, m.Text, m.FileSize, m.FileNameWithoutExtension, m.FileExtension, m.FileDirectory, m.State, m.Read))
	if e != nil {
		return e
	}

	return nil
}

// ChatMessageInfoUpdateState 更新会话消息状态
func ChatMessageInfoUpdateState(id int64, state string) error {
	_, e := db.Exec(fmt.Sprintf(`update chat_message set state = '%s' where id = %d`, state, id))
	if e != nil {
		return e
	}

	return nil
}

// ChatMessageInfoUpdateRead 通过节点ID更新会话消息已读状态
func ChatMessageInfoUpdateRead(peerID string, read bool) error {
	_, e := db.Exec(fmt.Sprintf(`update chat_message set read = %v where fromPeerID = '%s'`, read, peerID))
	if e != nil {
		return e
	}

	return nil
}

// ChatMessageInfoFind 查询会话消息
func ChatMessageInfoFind(peerID string) (*[]ChatMessageInfo, error) {
	var dataArray []ChatMessageInfo

	sqlText := fmt.Sprintf(`select * from chat_message where fromPeerID = '%s' or toPeerID = '%s'`, peerID, peerID)
	rows, e := db.Query(sqlText)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var data ChatMessageInfo
		e = rows.Scan(&data.ID, &data.FromPeerID, &data.ToPeerID, &data.Text,
			&data.FileSize, &data.FileNameWithoutExtension, &data.FileExtension, &data.FileDirectory,
			&data.State, &data.Read)
		if e != nil {
			return nil, e
		}
		dataArray = append(dataArray, data)
	}

	return &dataArray, nil
}

// ChatMessageInfoDeleteByPeerID 通过节点ID删除会话消息
func ChatMessageInfoDeleteByPeerID(peerID string) error {
	sqlText := fmt.Sprintf(`delete from chat_message where fromPeerID = '%s' or toPeerID = '%s'`, peerID, peerID)
	_, e := db.Exec(sqlText)
	if e != nil {
		return e
	}

	return nil
}

// ChatMessageInfoGet 通过ID获取会话消息
func ChatMessageInfoGet(id int64) (*ChatMessageInfo, error) {
	var data ChatMessageInfo

	sqlText := fmt.Sprintf(`select * from chat_message where id = %d`, id)
	e := db.QueryRow(sqlText).Scan(&data.ID, &data.FromPeerID, &data.ToPeerID, &data.Text, &data.FileSize, &data.FileNameWithoutExtension, &data.FileExtension, &data.State, &data.Read)
	if e == sql.ErrNoRows {
		return nil, e
	}

	return &data, nil
}

// ChatMessageInfoDeleteByID 通过消息ID删除会话消息
func ChatMessageInfoDeleteByID(id int64) error {
	sqlText := fmt.Sprintf(`delete from chat_message where id = %d`, id)
	_, e := db.Exec(sqlText)
	if e != nil {
		return e
	}

	return nil
}

// ChatMessageInfoUnReadCount 未读会话消息数量(按节点ID统计的Map)
func ChatMessageInfoUnReadCount() (*map[string]int64, error) {
	dm := make(map[string]int64)

	sqlText := fmt.Sprintf(`select fromPeerID as id, count(*) as c from chat_message where read = 0 group by fromPeerID`)
	rows, e := db.Query(sqlText)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id string
		var c int64
		e = rows.Scan(&id, &c)
		if e != nil {
			return nil, e
		}
		dm[id] = c
	}

	return &dm, nil
}
