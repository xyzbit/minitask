package queue

import "testing"

type testItem struct {
	id   int
	data string
}

func (t testItem) GetUniKey() testItem {
	return testItem{id: t.id} // 只用 id 作为唯一键
}

func TestSet(t *testing.T) {
	t.Run("基础操作", func(t *testing.T) {
		s := make(set[int], 0)

		// 测试 insert 和 len
		s.insert(1)
		if s.len() != 1 {
			t.Errorf("期望长度为 1, 得到 %d", s.len())
		}

		// 测试 has
		if !s.has(1) {
			t.Error("期望找到元素 1")
		}
		if s.has(2) {
			t.Error("不应该找到元素 2")
		}

		// 测试重复 insert
		s.insert(1)
		if s.len() != 1 {
			t.Errorf("重复插入后期望长度为 1, 得到 %d", s.len())
		}

		// 测试 delete
		s.delete(1)
		if s.len() != 0 {
			t.Errorf("删除后期望长度为 0, 得到 %d", s.len())
		}
		if s.has(1) {
			t.Error("删除后不应该找到元素 1")
		}
	})

	t.Run("UniKey接口", func(t *testing.T) {
		s := make(set[testItem], 0)

		item1 := testItem{id: 1, data: "test1"}
		item2 := testItem{id: 1, data: "test2"} // 相同的 id

		s.insert(item1)
		if s.len() != 1 {
			t.Errorf("期望长度为 1, 得到 %d", s.len())
		}

		// 测试具有相同 id 的项是否被视为相同
		if !s.has(item2) {
			t.Error("应该找到具有相同 id 的元素")
		}

		// 测试 delete 使用 UniKey
		s.delete(item2)
		if s.len() != 0 {
			t.Errorf("删除后期望长度为 0, 得到 %d", s.len())
		}
	})
}
