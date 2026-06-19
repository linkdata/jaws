// Package main implements the JaWS Minesweeper demo.
//
// The demo keeps all game state on the server and uses JaWS tags to refresh only
// the cells and status fields affected by each move. The implementation is kept
// in one file so it remains copyable as an application-wiring example; tests
// cover the game rules, dirty-targeting behavior, and HTTP handler wiring.
package main
