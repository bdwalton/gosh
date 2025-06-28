// Copyright (c) 2025, Ben Walton
// All rights reserved.
package vt

import (
	"testing"
)

var tNoMargin = &Terminal{
	fb: newFramebuffer(24, 80),
}
var tHorizMargin = &Terminal{
	fb:          newFramebuffer(24, 80),
	horizMargin: newMargin(5, 15),
}
var tVertMargin = &Terminal{
	fb:         newFramebuffer(24, 80),
	vertMargin: newMargin(5, 15),
}

var tVertWithDECOM = &Terminal{
	fb:         newFramebuffer(24, 80),
	vertMargin: newMargin(5, 15),
	modes: map[string]*mode{
		"?6": &mode{state: CSI_MODE_SET, code: DECOM},
	},
}

var tMarginsWithDECOM = &Terminal{
	fb:          newFramebuffer(24, 80),
	vertMargin:  newMargin(5, 15),
	horizMargin: newMargin(5, 15),
	modes: map[string]*mode{
		"?6": &mode{state: CSI_MODE_SET, code: DECOM},
	},
}

var minCol = 0
var minRow = 0
var maxCol = tNoMargin.rightMargin()
var maxRow = tNoMargin.bottomMargin()
var minVMargRow = tVertMargin.topMargin()
var maxVMargRow = tVertMargin.bottomMargin()
var minHMargCol = tHorizMargin.leftMargin()
var maxHMargCol = tHorizMargin.rightMargin()

var homeCursor = cursor{0, 0}
var midCur = cursor{15, 5}
var bottomCur = cursor{maxRow, 15}
var farRightCur = cursor{15, maxCol}
var farLeftCur = cursor{15, minCol}
var belowVertCur = cursor{16, minCol}

func TestCursorVPA(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// VPA - vertical position absolute
		{tNoMargin, farRightCur, 0, 0, maxCol},
		{tNoMargin, farRightCur, 1, 1, maxCol},
		{tNoMargin, farRightCur, 9, 9, maxCol},
		{tNoMargin, farLeftCur, 3, 3, 0},
		{tNoMargin, farLeftCur, 1000, maxRow, 0},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorVPA(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorHPR(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// HPR - horizontal position relative
		{tNoMargin, farRightCur, 0, 15, maxCol},
		{tNoMargin, midCur, 1, 15, 6},
		{tNoMargin, farLeftCur, 1, 15, 1},
		{tNoMargin, midCur, 10, 15, 15},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorHPR(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorVPR(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// VPR - vertical position relative
		{tNoMargin, farRightCur, 0, 15, maxCol},
		{tNoMargin, farRightCur, 1, 16, maxCol},
		{tNoMargin, farRightCur, 5, 20, 79},
		{tNoMargin, farLeftCur, 3, 18, 0},
		{tNoMargin, farLeftCur, 1000, maxRow, 0},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorVPR(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCHAorHPA(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// HPA - horizontal position absolute
		{tNoMargin, farRightCur, 0, 15, 0},
		{tNoMargin, farRightCur, 10, 15, 10},
		{tNoMargin, farLeftCur, 3, 15, 3},
		{tNoMargin, farLeftCur, 1000, 15, maxCol},
		{tNoMargin, homeCursor, 0, 0, 0},
		{tNoMargin, homeCursor, 1, 0, 1},
		{tNoMargin, homeCursor, 10, 0, 10},
		{tNoMargin, midCur, 10, 15, 10},
		{tNoMargin, midCur, maxCol + 10, 15, maxCol},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorCHAorHPA(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCUPorHVP(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		m, n             int
		wantRow, wantCol int
	}{
		// CUP - cursor position
		{tNoMargin, homeCursor, 0, 0, 0, 0},
		{tNoMargin, midCur, 0, 0, 0, 0},
		{tNoMargin, midCur, 0, 25, 0, 25},
		{tNoMargin, midCur, 16, 0, 16, 0},
		{tNoMargin, midCur, 21, 16, 21, 16},
		{tNoMargin, midCur, 1000, 1000, maxRow, maxCol},
		{tNoMargin, midCur, 1000, 0, maxRow, 0},
		{tNoMargin, midCur, 0, 1000, 0, maxCol},
		{tVertMargin, midCur, 0, 0, 0, 0},          // no origin mode, so still home
		{tVertWithDECOM, midCur, 0, 0, 5, 0},       // origin mode, so home in region
		{tMarginsWithDECOM, midCur, 0, 0, 5, 5},    // origin mode, home in region
		{tMarginsWithDECOM, bottomCur, 0, 0, 0, 0}, // origin mode, outside region
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorCUPorHVP(c.m, c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorUp(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// CUU - cursor up
		{tNoMargin, homeCursor, 1, 0, 0},
		{tNoMargin, homeCursor, 2, 0, 0},
		{tNoMargin, midCur, 1, 14, 5},
		{tNoMargin, midCur, 2, 13, 5},
		{tVertMargin, midCur, 2, 13, 5},
		{tVertMargin, midCur, 11, minVMargRow, 5},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorUp(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorDown(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// CUD - cursor down
		{tNoMargin, bottomCur, 0, maxRow, 15},
		{tNoMargin, midCur, 1, 16, 5},
		{tNoMargin, bottomCur, 1, maxRow, 15},
		{tNoMargin, bottomCur, 3, maxRow, 15},
		{tNoMargin, homeCursor, 1, 1, 0},
		{tNoMargin, homeCursor, 3, 3, 0},
		{tVertMargin, midCur, 2, 15, 5},
		{tVertMargin, midCur, 3, 15, 5},
		{tVertMargin, belowVertCur, 2, 18, 0},
		{tVertMargin, midCur, 6, 15, 5},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorDown(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorBack(t *testing.T) {

	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// CUB - cursor back
		{tNoMargin, cursor{15, 15}, 1, 15, 14},
		{tNoMargin, cursor{15, 0}, 1, 15, 0},
		{tNoMargin, cursor{15, 0}, 2, 15, 0},
		{tNoMargin, cursor{15, 3}, 2, 15, 1},
		{tNoMargin, cursor{15, 79}, 1, 15, 78},
		{tHorizMargin, cursor{15, 0}, 2, 15, 0},
		{tHorizMargin, cursor{15, 5}, 2, 15, 5},
		{tHorizMargin, cursor{10, 5}, 6, 10, 5},
		{tHorizMargin, cursor{10, 4}, 2, 10, 2},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorBack(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorForward(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// CUF - cursor forward
		{tNoMargin, cursor{15, 0}, 1, 15, 1},
		{tNoMargin, cursor{15, 0}, 10, 15, 10},
		{tNoMargin, cursor{15, tNoMargin.Cols() - 1}, 0, 15, tNoMargin.Cols() - 1},
		{tNoMargin, cursor{15, tNoMargin.Cols() - 1}, 1, 15, tNoMargin.Cols() - 1},
		{tNoMargin, cursor{15, tNoMargin.Cols() - 1}, 10, 15, tNoMargin.Cols() - 1},
		{tHorizMargin, cursor{15, 0}, 2, 15, 2},
		{tHorizMargin, cursor{15, 5}, 2, 15, 7},
		{tHorizMargin, cursor{10, 10}, 6, 10, 15},
		{tHorizMargin, cursor{10, 16}, 2, 10, 18},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorForward(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCPL(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// CPL - previous line
		{tNoMargin, homeCursor, 1, 0, 0},
		{tNoMargin, homeCursor, 0, 0, 0},
		{tNoMargin, cursor{1, 0}, 0, 1, 0},
		{tNoMargin, cursor{0, 10}, 20, 0, 0},
		{tNoMargin, cursor{15, 20}, 1, 14, 0},
		{tNoMargin, cursor{21, 10}, 20, 1, 0},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorCPL(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}

func TestCursorCNL(t *testing.T) {
	cases := []struct {
		t                *Terminal
		cur              cursor
		n                int
		wantRow, wantCol int
	}{
		// CNL - next line
		{tNoMargin, homeCursor, 0, 0, 0},
		{tNoMargin, midCur, 0, 15, 0},
		{tNoMargin, bottomCur, 0, maxRow, 0},
		{tNoMargin, bottomCur, 1, maxRow, 0},
		{tNoMargin, bottomCur, 2, maxRow, 0},
		{tNoMargin, homeCursor, 1, 1, 0},
		{tNoMargin, homeCursor, 10, 10, 0},
	}

	for i, c := range cases {
		c.t.cur = c.cur
		c.t.cursorCNL(c.n)
		if row, col := c.t.row(), c.t.col(); col != c.wantCol || row != c.wantRow {
			t.Errorf("%d: Got (r: %d, c: %d), wanted (r: %d, c: %d)", i, row, col, c.wantRow, c.wantCol)
		}
	}
}
