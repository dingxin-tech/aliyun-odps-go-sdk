package tunnel_test

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

// 旧版本实现（用于对比）
type OldProtocStreamWriter struct {
	inner io.Writer
}

func (r *OldProtocStreamWriter) WriteVarint(v uint64) error {
	b := make([]byte, 0, 1)
	b = protowire.AppendVarint(b, v)
	_, err := io.Copy(r.inner, bytes.NewReader(b))
	return err
}

func (r *OldProtocStreamWriter) WriteFixed32(val uint32) error {
	b := make([]byte, 0, 4)
	b = protowire.AppendFixed32(b, val)
	_, err := io.Copy(r.inner, bytes.NewReader(b))
	return err
}

func (r *OldProtocStreamWriter) WriteFixed64(val uint64) error {
	b := make([]byte, 0, 8)
	b = protowire.AppendFixed64(b, val)
	_, err := io.Copy(r.inner, bytes.NewReader(b))
	return err
}

func (r *OldProtocStreamWriter) WriteBytes(b []byte) error {
	err := r.WriteVarint(uint64(len(b)))
	if err != nil {
		return err
	}

	_, err = io.Copy(r.inner, bytes.NewReader(b))
	return err
}

// 新版本实现
type NewProtocStreamWriter struct {
	inner io.Writer
}

func (r *NewProtocStreamWriter) WriteVarint(v uint64) error {
	b := protowire.AppendVarint(nil, v)
	return writeFull(r.inner, b)
}

func (r *NewProtocStreamWriter) WriteFixed32(val uint32) error {
	b := protowire.AppendFixed32(nil, val)
	return writeFull(r.inner, b)
}

func (r *NewProtocStreamWriter) WriteFixed64(val uint64) error {
	b := protowire.AppendFixed64(nil, val)
	return writeFull(r.inner, b)
}

func (r *NewProtocStreamWriter) WriteBytes(data []byte) error {
	if err := r.WriteVarint(uint64(len(data))); err != nil {
		return err
	}
	err := writeFull(r.inner, data)
	return err
}

// 公共 writeFull 实现
func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

var bufPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 10)
	},
}

// 新版本实现
type NewProtocStreamWriter2 struct {
	inner io.Writer
}

func (r *NewProtocStreamWriter2) WriteVarint(v uint64) error {
	b := bufPool.Get().([]byte)
	b = protowire.AppendVarint(b, v)
	err := writeFull(r.inner, b)
	bufPool.Put(b[:0])
	return err
}

func (r *NewProtocStreamWriter2) WriteFixed32(val uint32) error {
	b := bufPool.Get().([]byte)
	b = protowire.AppendFixed32(b, val)
	err := writeFull(r.inner, b)
	bufPool.Put(b[:0])
	return err
}

func (r *NewProtocStreamWriter2) WriteFixed64(val uint64) error {
	b := bufPool.Get().([]byte)
	b = protowire.AppendFixed64(b, val)
	err := writeFull(r.inner, b)
	bufPool.Put(b[:0])
	return err
}

func (r *NewProtocStreamWriter2) WriteBytes(data []byte) error {
	if err := r.WriteVarint(uint64(len(data))); err != nil {
		return err
	}
	err := writeFull(r.inner, data)
	return err
}

// | 测试用例             | 旧实现 (ns/op)  | 新实现 (ns/op)  | 时间提升   | 旧内存 (B/op)  | 新内存 (B/op)  | 内存优化  | 旧分配次数   | 新分配次数   | 分配优化 |
// |---------------------|----------------|----------------|----------|---------------|---------------|----------|------------|------------|----------|
// | **Varint_Small**    | 35.89          | 17.01          | 52.6%    | 49            | 8             | 83.7%    | 2          | 1          | 50.0%    |
// | **Varint_Medium**   | 47.11          | 16.82          | 64.3%    | 64            | 8             | 87.5%    | 3          | 1          | 66.7%    |
// | **Varint_Large**    | 50.74          | 19.98          | 60.6%    | 64            | 8             | 87.5%    | 3          | 1          | 66.7%    |
// | **Fixed32**         | 32.83          | 15.68          | 52.2%    | 52            | 8             | 84.6%    | 2          | 1          | 50.0%    |
// | **Fixed64**         | 33.30          | 16.50          | 50.4%    | 56            | 8             | 85.7%    | 2          | 1          | 50.0%    |
// | **WriteBytes_Small**| 65.82          | 28.72          | 56.4%    | 104           | 16            | 84.6%    | 4          | 2          | 50.0%    |
// 正确性测试
// 定义统一接口类型
type writerInterface interface {
	WriteVarint(uint64) error
	WriteFixed32(uint32) error
	WriteFixed64(uint64) error
	WriteBytes([]byte) error
}

func TestCorrectness(t *testing.T) {
	testCases := []struct {
		name string
		run  func(writer writerInterface) error
	}{
		{"WriteVarint", func(w writerInterface) error {
			return w.WriteVarint(12345)
		}},
		{"WriteFixed32", func(w writerInterface) error {
			return w.WriteFixed32(0x12345678)
		}},
		{"WriteFixed64", func(w writerInterface) error {
			return w.WriteFixed64(0x1234567890ABCDEF)
		}},
		{"WriteBytes", func(w writerInterface) error {
			return w.WriteBytes([]byte("test data"))
		}},
	}

	for _, tc := range testCases {
		oldBuf := &bytes.Buffer{}
		oldWriter := &OldProtocStreamWriter{inner: oldBuf}
		err := tc.run(oldWriter)
		if err != nil {
			t.Errorf("Old %s failed: %v", tc.name, err)
		}

		newBuf := &bytes.Buffer{}
		newWriter := &NewProtocStreamWriter{inner: newBuf}
		err = tc.run(newWriter)
		if err != nil {
			t.Errorf("New %s failed: %v", tc.name, err)
		}

		if !bytes.Equal(oldBuf.Bytes(), newBuf.Bytes()) {
			t.Errorf("%s: outputs differ. Old: %x, New: %x", tc.name, oldBuf.Bytes(), newBuf.Bytes())
		}
	}
}

// 基准测试工具函数
func runBenchmark(b *testing.B, writer interface {
	WriteVarint(uint64) error
	WriteFixed32(uint32) error
	WriteFixed64(uint64) error
	WriteBytes([]byte) error
},
) {
	testCases := []struct {
		name string
		fn   func() error
	}{
		{"Varint_Small", func() error { return writer.WriteVarint(127) }},
		{"Varint_Medium", func() error { return writer.WriteVarint(16383) }},
		{"Varint_Large", func() error { return writer.WriteVarint(1<<56 - 1) }},
		{"Fixed32", func() error { return writer.WriteFixed32(0x12345678) }},
		{"Fixed64", func() error { return writer.WriteFixed64(0x1234567890ABCDEF) }},
		{"WriteBytes_Small", func() error { return writer.WriteBytes([]byte("small")) }},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := tc.fn(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// 新旧实现对比基准测试
// BenchmarkOldImplementation
// BenchmarkOldImplementation/Varint_Small
// BenchmarkOldImplementation/Varint_Small-8         	34086914	        34.28 ns/op	      49 B/op	       2 allocs/op
// BenchmarkOldImplementation/Varint_Medium
// BenchmarkOldImplementation/Varint_Medium-8        	23941263	        50.77 ns/op	      64 B/op	       3 allocs/op
// BenchmarkOldImplementation/Varint_Large
// BenchmarkOldImplementation/Varint_Large-8         	19006237	        54.12 ns/op	      64 B/op	       3 allocs/op
// BenchmarkOldImplementation/Fixed32
// BenchmarkOldImplementation/Fixed32-8              	32886622	        35.90 ns/op	      52 B/op	       2 allocs/op
// BenchmarkOldImplementation/Fixed64
// BenchmarkOldImplementation/Fixed64-8              	33470197	        33.28 ns/op	      56 B/op	       2 allocs/op
// BenchmarkOldImplementation/WriteBytes_Small
// BenchmarkOldImplementation/WriteBytes_Small-8     	17467216	        66.81 ns/op	     104 B/op	       4 allocs/op
func BenchmarkOldImplementation(b *testing.B) {
	oldWriter := &OldProtocStreamWriter{inner: io.Discard}
	runBenchmark(b, oldWriter)
}

// BenchmarkNewImplementation
// BenchmarkNewImplementation/Varint_Small
// BenchmarkNewImplementation/Varint_Small-8         	66530526	        17.62 ns/op	       8 B/op	       1 allocs/op
// BenchmarkNewImplementation/Varint_Medium
// BenchmarkNewImplementation/Varint_Medium-8        	69088772	        17.39 ns/op	       8 B/op	       1 allocs/op
// BenchmarkNewImplementation/Varint_Large
// BenchmarkNewImplementation/Varint_Large-8         	59353785	        19.72 ns/op	       8 B/op	       1 allocs/op
// BenchmarkNewImplementation/Fixed32
// BenchmarkNewImplementation/Fixed32-8              	73491507	        16.10 ns/op	       8 B/op	       1 allocs/op
// BenchmarkNewImplementation/Fixed64
// BenchmarkNewImplementation/Fixed64-8              	70767063	        17.64 ns/op	       8 B/op	       1 allocs/op
// BenchmarkNewImplementation/WriteBytes_Small
// BenchmarkNewImplementation/WriteBytes_Small-8     	39911085	        29.25 ns/op	      16 B/op	       2 allocs/op
func BenchmarkNewImplementation(b *testing.B) {
	newWriter := &NewProtocStreamWriter{inner: io.Discard}
	runBenchmark(b, newWriter)
}

// BenchmarkNew2Implementation
// BenchmarkNew2Implementation/Varint_Small
// BenchmarkNew2Implementation/Varint_Small-8         	31800223	        33.69 ns/op	      24 B/op	       1 allocs/op
// BenchmarkNew2Implementation/Varint_Medium
// BenchmarkNew2Implementation/Varint_Medium-8        	29627342	        34.20 ns/op	      24 B/op	       1 allocs/op
// BenchmarkNew2Implementation/Varint_Large
// BenchmarkNew2Implementation/Varint_Large-8         	32771252	        35.26 ns/op	      24 B/op	       1 allocs/op
// BenchmarkNew2Implementation/Fixed32
// BenchmarkNew2Implementation/Fixed32-8              	36227144	        33.78 ns/op	      24 B/op	       1 allocs/op
// BenchmarkNew2Implementation/Fixed64
// BenchmarkNew2Implementation/Fixed64-8              	33905568	        31.97 ns/op	      24 B/op	       1 allocs/op
// BenchmarkNew2Implementation/WriteBytes_Small
// BenchmarkNew2Implementation/WriteBytes_Small-8     	21873462	        50.50 ns/op	      29 B/op	       2 allocs/op
func BenchmarkNew2Implementation(b *testing.B) {
	newWriter := &NewProtocStreamWriter2{inner: io.Discard}
	runBenchmark(b, newWriter)
}

// WriteBytes 专项测试
// BenchmarkWriteBytes
// BenchmarkWriteBytes/Old_Small
// BenchmarkWriteBytes/Old_Small-8         	15771470	        72.60 ns/op	     112 B/op	       4 allocs/op
// BenchmarkWriteBytes/New_Small
// BenchmarkWriteBytes/New_Small-8         	58773110	        19.97 ns/op	       8 B/op	       1 allocs/op
// BenchmarkWriteBytes/New2_Small
// BenchmarkWriteBytes/New2_Small-8        	36471883	        32.51 ns/op	      24 B/op	       1 allocs/op
// BenchmarkWriteBytes/Old_1KB
// BenchmarkWriteBytes/Old_1KB-8           	16531686	        70.70 ns/op	     112 B/op	       4 allocs/op
// BenchmarkWriteBytes/New_1KB
// BenchmarkWriteBytes/New_1KB-8           	58092157	        19.92 ns/op	       8 B/op	       1 allocs/op
// BenchmarkWriteBytes/New2_1KB
// BenchmarkWriteBytes/New2_1KB-8          	35389356	        32.52 ns/op	      24 B/op	       1 allocs/op
// BenchmarkWriteBytes/Old_1KB#01
// BenchmarkWriteBytes/Old_1KB#01-8        	16017529	        72.76 ns/op	     112 B/op	       4 allocs/op
// BenchmarkWriteBytes/New_1KB#01
// BenchmarkWriteBytes/New_1KB#01-8        	57669663	        20.17 ns/op	       8 B/op	       1 allocs/op
// BenchmarkWriteBytes/New2_1KB#01
// BenchmarkWriteBytes/New2_1KB#01-8       	36332670	        32.85 ns/op	      24 B/op	       1 allocs/op
func BenchmarkWriteBytes(b *testing.B) {
	sizes := []int{
		128,       // 小数据
		1024 * 4,  // 4KB
		1024 * 64, // 64KB
	}

	for _, size := range sizes {
		data := make([]byte, size)
		name := byteSizeLabel(size)

		b.Run("Old_"+name, func(b *testing.B) {
			oldWriter := &OldProtocStreamWriter{inner: io.Discard}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := oldWriter.WriteBytes(data); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("New_"+name, func(b *testing.B) {
			newWriter := &NewProtocStreamWriter{inner: io.Discard}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := newWriter.WriteBytes(data); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("New2_"+name, func(b *testing.B) {
			newWriter := &NewProtocStreamWriter2{inner: io.Discard}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := newWriter.WriteBytes(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func byteSizeLabel(size int) string {
	switch {
	case size >= 1<<20:
		return "1MB"
	case size >= 1<<10:
		return "1KB"
	default:
		return "Small"
	}
}
