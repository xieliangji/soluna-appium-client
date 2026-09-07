package appium

// BelongsTo 报告 Element 是否归属于给定的 Session。
//
// 本方法只查询本地对象关系，不发送远端请求。Session 值复制保留归属，
// 独立创建的 Session 即使远端 ID 相同也不视为同一个 Session。
// 调用方重新赋值 Session 变量不会改变已创建 Element 的归属。
// nil 或未初始化的 Element、Session 返回 false。
// Session 关闭不改变归属；返回 true 不保证 Session 可用或 Element 未失效。
func (e *Element) BelongsTo(session *Session) bool {
	if e == nil ||
		e.session == nil ||
		e.session.state == nil ||
		e.session.client == nil ||
		e.session.id == "" ||
		e.id == "" ||
		session == nil ||
		session.state == nil ||
		session.client == nil ||
		session.id == "" {
		return false
	}

	return e.session.state == session.state &&
		e.session.client == session.client &&
		e.session.id == session.id
}
