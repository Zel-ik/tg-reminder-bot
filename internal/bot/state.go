package bot

import "sync"

type State struct {
	Step string                 // текущий шаг (например, "create_waiting_schedule")
	Data map[string]interface{} // временные данные
}

var (
	userStates = make(map[int64]*State)
	stateMu    sync.RWMutex
)

func SetUserState(userID int64, state *State) {
	stateMu.Lock()
	defer stateMu.Unlock()
	userStates[userID] = state
}

func GetUserState(userID int64) *State {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return userStates[userID]
}

func ClearUserState(userID int64) {
	stateMu.Lock()
	defer stateMu.Unlock()
	delete(userStates, userID)
}
