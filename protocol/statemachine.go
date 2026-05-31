package protocol

import "fmt"

// StateMachine is a generic finite-state machine.
// S is the state type, which must be comparable.
type StateMachine[S comparable] struct {
	current     S
	transitions map[S]map[S]struct{} // from -> set of valid to's
	onEnter     map[S]func(from S)
	onExit      map[S]func(to S)
}

// NewStateMachine creates a new state machine starting in the given state.
func NewStateMachine[S comparable](initial S) *StateMachine[S] {
	return &StateMachine[S]{
		current:     initial,
		transitions: make(map[S]map[S]struct{}),
		onEnter:     make(map[S]func(from S)),
		onExit:      make(map[S]func(to S)),
	}
}

// Allow registers a valid transition from -> to.
func (sm *StateMachine[S]) Allow(from, to S) *StateMachine[S] {
	if sm.transitions[from] == nil {
		sm.transitions[from] = make(map[S]struct{})
	}
	sm.transitions[from][to] = struct{}{}
	return sm
}

// OnEnter registers a callback invoked when entering the given state.
func (sm *StateMachine[S]) OnEnter(state S, fn func(from S)) *StateMachine[S] {
	sm.onEnter[state] = fn
	return sm
}

// OnExit registers a callback invoked when exiting the given state.
func (sm *StateMachine[S]) OnExit(state S, fn func(to S)) *StateMachine[S] {
	sm.onExit[state] = fn
	return sm
}

// CanTransition reports whether a transition to the given state is allowed.
func (sm *StateMachine[S]) CanTransition(to S) bool {
	_, ok := sm.transitions[sm.current][to]
	return ok
}

// Transition attempts to move to the given state.
// It returns an error if the transition is not allowed.
func (sm *StateMachine[S]) Transition(to S) error {
	if !sm.CanTransition(to) {
		return fmt.Errorf("invalid transition: %v -> %v", sm.current, to)
	}
	from := sm.current
	if fn := sm.onExit[from]; fn != nil {
		fn(to)
	}
	sm.current = to
	if fn := sm.onEnter[to]; fn != nil {
		fn(from)
	}
	return nil
}

// Current returns the current state.
func (sm *StateMachine[S]) Current() S {
	return sm.current
}
