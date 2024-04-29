package storage

import (
	"fmt"
	"sort"
)

const (
	float64size = 64
	maxInt      = int(^uint(0) >> 1)
	ptrSize     = 32 << (^uintptr(0) >> 63)
)

type BufferedPaginatedStorage struct {
	buffer                []int
	compactionTriggerSize int
	pages                 [][]float64
	minPageIndex          int
	pageLenLog2           int
	pageLenMask           int
	bufferEntrySize       int
}

func NewBufferedPaginatedStorage() *BufferedPaginatedStorage {
	return &BufferedPaginatedStorage{
		buffer:                make([]int, 0, 4),
		compactionTriggerSize: 64,
		minPageIndex:          maxInt,
		pageLenLog2:           5,
		pageLenMask:           31,
		bufferEntrySize:       32 << (^uint(0) >> 63),
	}
}

func (b *BufferedPaginatedStorage) RecordValue(index int, count float64) {
	if count == 0 {
		return
	} else if count == 1 {
		b.add(index)
	} else {
		b.page(b.pageIndex(index), true)[b.lineIndex(index)] += count
	}
}

func (b *BufferedPaginatedStorage) Size() float64 {
	totalCount := float64(len(b.buffer))
	for _, page := range b.pages {
		for _, count := range page {
			totalCount += count
		}
	}
	return totalCount
}

func Merge(left *BufferedPaginatedStorage, right *BufferedPaginatedStorage) (*BufferedPaginatedStorage, error) {
	if left.pageLenLog2 != right.pageLenLog2 {
		return nil, fmt.Errorf("storages cannot be merged")
	}
	for rightOffset, rightPage := range right.pages {
		if len(rightPage) == 0 {
			continue
		}
		rightPageIndex := right.minPageIndex + rightOffset
		leftPage := left.page(rightPageIndex, true)
		for rightIndex, rightCount := range rightPage {
			leftPage[rightIndex] += rightCount
		}
		for _, rightIndex := range right.buffer {
			left.add(rightIndex)
		}
	}
	return left, nil
}

func (b *BufferedPaginatedStorage) KeyAtRank(rank float64) (int, error) {
	rank = max(0, rank)
	key, err := b.minIndexWithCumulativeCount(rank)
	if err != nil {
		maxIndex, err := b.maxIndex()
		if err != nil {
			return 0, err
		}
		return maxIndex, nil
	}
	return key, nil
}

func (b *BufferedPaginatedStorage) maxIndex() (int, error) {
	isEmpty := true
	var maxIndex int

	for _, index := range b.buffer {
		if isEmpty || index > maxIndex {
			isEmpty = false
			maxIndex = index
		}
	}

	for pageIndex := b.minPageIndex + len(b.pages) - 1; pageIndex >= b.minPageIndex && (isEmpty || pageIndex >= b.pageIndex(maxIndex)); pageIndex-- {
		page := b.pages[pageIndex-b.minPageIndex]
		if len(page) == 0 {
			continue
		}
		var lineIndexRangeStart int
		if !isEmpty && pageIndex == b.pageIndex(maxIndex) {
			lineIndexRangeStart = b.lineIndex(maxIndex)
		} else {
			lineIndexRangeStart = 0
		}
		for lineIndex := len(page) - 1; lineIndex >= lineIndexRangeStart; lineIndex-- {
			if page[lineIndex] > 0 {
				return b.index(pageIndex, lineIndex), nil
			}
		}
	}
	if isEmpty {
		return 0, fmt.Errorf("max index not found")
	}
	return maxIndex, nil
}

func (b *BufferedPaginatedStorage) minIndexWithCumulativeCount(rank float64) (int, error) {
	b.sortBuffer()

	var cumulativeCount float64
	var bufferPosition int

	for pageOffset, page := range b.pages {
		for lineIndex, count := range page {
			index := b.index(b.minPageIndex+pageOffset, lineIndex)
			for ; bufferPosition < len(b.buffer) && b.buffer[bufferPosition] < index; bufferPosition++ {
				cumulativeCount++
				if cumulativeCount > rank {
					return b.buffer[bufferPosition], nil
				}
			}
			cumulativeCount += count
			if cumulativeCount > rank {
				return index, nil
			}
		}
	}
	for ; bufferPosition < len(b.buffer); bufferPosition++ {
		cumulativeCount++
		if cumulativeCount > rank {
			return b.buffer[bufferPosition], nil
		}
	}
	return 0, fmt.Errorf("min index with cumulative rank not found")
}

func (b *BufferedPaginatedStorage) pageIndex(index int) int {
	return index >> b.pageLenLog2
}

func (b *BufferedPaginatedStorage) lineIndex(index int) int {
	return index & b.pageLenMask
}

func (b *BufferedPaginatedStorage) index(pageIndex int, lineIndex int) int {
	return pageIndex<<b.pageLenLog2 + lineIndex
}

func (b *BufferedPaginatedStorage) sortBuffer() {
	sort.Ints(b.buffer)
}

func (b *BufferedPaginatedStorage) compact() {
	pageSize := 1 << b.pageLenLog2

	b.sortBuffer()

	for bufferPosition := 0; bufferPosition < len(b.buffer); {
		bufferPageStart := bufferPosition
		pageIndex := b.pageIndex(b.buffer[bufferPageStart])
		bufferPosition++
		for bufferPosition < len(b.buffer) && b.pageIndex(b.buffer[bufferPosition]) == pageIndex {
			bufferPosition++
		}
		bufferPageEnd := bufferPosition
		ensureExists := (bufferPageEnd-bufferPageStart)*b.bufferEntrySize >= pageSize*float64size
		newPage := b.page(pageIndex, ensureExists)
		if len(newPage) > 0 {
			for _, index := range b.buffer[bufferPageStart:bufferPageEnd] {
				newPage[b.lineIndex(index)]++
			}
			copy(b.buffer[bufferPageStart:], b.buffer[bufferPageEnd:])
			b.buffer = b.buffer[:len(b.buffer)+bufferPageStart-bufferPageEnd]
			bufferPosition = bufferPageStart
		}
	}
	b.compactionTriggerSize = len(b.buffer) + pageSize
}

func (b *BufferedPaginatedStorage) page(pageIndex int, ensureExists bool) []float64 {
	pageLen := 1 << b.pageLenLog2

	if pageIndex >= b.minPageIndex && pageIndex < b.minPageIndex+len(b.pages) {
		page := &b.pages[pageIndex-b.minPageIndex]
		if ensureExists && len(*page) == 0 {
			*page = append(*page, make([]float64, pageLen)...)
		}
		return *page
	}

	if !ensureExists {
		return nil
	}

	if pageIndex < b.minPageIndex {
		if b.minPageIndex == maxInt {
			if len(b.pages) == 0 {
				b.pages = append(b.pages, make([][]float64, b.newPagesSize(1))...)
			}
			b.minPageIndex = pageIndex - len(b.pages)/2
		} else {
			newLen := b.newPagesSize(b.minPageIndex - pageIndex + 1 + len(b.pages))
			addedLen := newLen - len(b.pages)
			b.pages = append(b.pages, make([][]float64, addedLen)...)
			copy(b.pages[addedLen:], b.pages)
			for i := 0; i < addedLen; i++ {
				b.pages[i] = nil
			}
			b.minPageIndex -= addedLen
		}
	} else {
		b.pages = append(b.pages, make([][]float64, b.newPagesSize(pageIndex-b.minPageIndex+1)-len(b.pages))...)
	}

	page := &b.pages[pageIndex-b.minPageIndex]
	if len(*page) == 0 {
		*page = append(*page, make([]float64, pageLen)...)
	}
	return *page
}

func (b *BufferedPaginatedStorage) newPagesSize(required int) int {
	growthIncrement := 64 * 8 / ptrSize
	return (required + growthIncrement - 1) & -growthIncrement
}

func (b *BufferedPaginatedStorage) add(index int) {
	pageIndex := b.pageIndex(index)
	if pageIndex >= b.minPageIndex && pageIndex < b.minPageIndex+len(b.pages) {
		page := b.pages[pageIndex-b.minPageIndex]
		if len(page) > 0 {
			page[b.lineIndex(index)]++
			return
		}
	}
	if len(b.buffer) == cap(b.buffer) && len(b.buffer) >= b.compactionTriggerSize {
		b.compact()
	}
	b.buffer = append(b.buffer, index)
}
