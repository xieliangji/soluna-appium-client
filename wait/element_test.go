package wait_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	appium "github.com/xieliangji/soluna-appium-client"
	"github.com/xieliangji/soluna-appium-client/wait"
)

func TestElementRetriesNotFoundAndReturnsElement(t *testing.T) {
	firstErr := &appium.Error{
		Code:      appium.CodeElementNotFound,
		Operation: "find_element",
		Message:   "not present yet",
		Delivery:  appium.DeliveryAcknowledged,
	}
	finder := &stubFinder{
		elementResults: []elementResult{
			{err: firstErr},
			{element: &appium.Element{}},
		},
	}

	const interval = 5 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	started := time.Now()
	element, err := wait.Element(
		ctx,
		interval,
		finder,
		appium.ID("login"),
	)
	if err != nil {
		t.Fatalf("Element() error = %v", err)
	}
	if element == nil {
		t.Fatal("Element() returned nil element")
	}

	if got := finder.elementCallCount(); got != 2 {
		t.Fatalf("Find calls = %d, want 2", got)
	}

	callTimes := finder.elementCallTimes()
	if got := callTimes[1].Sub(callTimes[0]); got < interval {
		t.Fatalf("poll interval = %v, want at least %v", got, interval)
	}
	if time.Since(started) < interval {
		t.Fatalf("Element() returned before retry interval")
	}

	if got := finder.locators()[1]; got != (appium.Locator{Strategy: appium.StrategyID, Value: "login"}) {
		t.Fatalf("locator = %+v, want id locator", got)
	}
}

func TestElementStopsOnNonTransientError(t *testing.T) {
	wantErr := &appium.Error{
		Code:      appium.CodeElementStale,
		Operation: "find_element",
		Message:   "stale",
		Delivery:  appium.DeliveryAcknowledged,
	}
	finder := &stubFinder{
		elementResults: []elementResult{{err: wantErr}},
	}

	element, err := wait.Element(
		context.Background(),
		time.Hour,
		finder,
		appium.ID("login"),
	)
	if element != nil {
		t.Fatalf("Element() returned element on failure: %v", element)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Element() error = %v, want %v", err, wantErr)
	}
	if got := finder.elementCallCount(); got != 1 {
		t.Fatalf("Find calls = %d, want 1", got)
	}
}

func TestElementDeadlineKeepsContextAndLastNotFound(t *testing.T) {
	firstErr := &appium.Error{
		Code:      appium.CodeElementNotFound,
		Operation: "find_element",
		Message:   "not present",
		Delivery:  appium.DeliveryAcknowledged,
	}
	finder := &stubFinder{
		elementResults: []elementResult{{err: firstErr}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	element, err := wait.Element(
		ctx,
		2*time.Millisecond,
		finder,
		appium.ID("login"),
	)
	if element != nil {
		t.Fatalf("Element() returned element after deadline: %v", element)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Element() error = %v, want context deadline", err)
	}
	if !appium.IsErrorCode(err, appium.CodeElementNotFound) {
		t.Fatalf("Element() error = %v, want final not-found error", err)
	}
	if got := finder.elementCallCount(); got < 2 {
		t.Fatalf("Find calls = %d, want at least 2", got)
	}
}

func TestElementsRetriesEmptyAndNotFound(t *testing.T) {
	notFound := &appium.Error{
		Code:      appium.CodeElementNotFound,
		Operation: "find_elements",
		Message:   "not present yet",
		Delivery:  appium.DeliveryAcknowledged,
	}
	want := []*appium.Element{{}}
	finder := &stubFinder{
		elementsResults: []elementsResult{
			{elements: []*appium.Element{}},
			{err: notFound},
			{elements: want},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	elements, err := wait.Elements(
		ctx,
		2*time.Millisecond,
		finder,
		appium.XPath("//button"),
	)
	if err != nil {
		t.Fatalf("Elements() error = %v", err)
	}
	if len(elements) != 1 || elements[0] != want[0] {
		t.Fatalf("Elements() = %#v, want %#v", elements, want)
	}
	if got := finder.elementsCallCount(); got != 3 {
		t.Fatalf("FindElements calls = %d, want 3", got)
	}
}

func TestElementsStopsOnNonTransientError(t *testing.T) {
	wantErr := &appium.Error{
		Code:      appium.CodeSessionLost,
		Operation: "find_elements",
		Message:   "session closed",
		Delivery:  appium.DeliveryAcknowledged,
	}
	finder := &stubFinder{
		elementsResults: []elementsResult{{err: wantErr}},
	}

	elements, err := wait.Elements(
		context.Background(),
		time.Hour,
		finder,
		appium.ID("missing"),
	)
	if elements != nil {
		t.Fatalf("Elements() returned elements on failure: %v", elements)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Elements() error = %v, want %v", err, wantErr)
	}
	if got := finder.elementsCallCount(); got != 1 {
		t.Fatalf("FindElements calls = %d, want 1", got)
	}
}

func TestElementsRejectsNilElementInSuccessfulCollection(t *testing.T) {
	finder := &stubFinder{
		elementsResults: []elementsResult{
			{elements: []*appium.Element{nil}},
		},
	}

	elements, err := wait.Elements(
		context.Background(),
		time.Millisecond,
		finder,
		appium.ID("item"),
	)
	if elements != nil {
		t.Fatalf("Elements() returned malformed elements: %v", elements)
	}
	if !appium.IsErrorCode(err, appium.CodeResponseInvalid) {
		t.Fatalf("Elements() error = %v, want response-invalid", err)
	}
	if got := finder.elementsCallCount(); got != 1 {
		t.Fatalf("FindElements calls = %d, want 1", got)
	}
}

func TestElementsDeadlineAfterOnlyEmptyResultsReturnsContextError(t *testing.T) {
	finder := &stubFinder{
		elementsResults: []elementsResult{{elements: []*appium.Element{}}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	elements, err := wait.Elements(
		ctx,
		2*time.Millisecond,
		finder,
		appium.ID("missing"),
	)
	if elements != nil {
		t.Fatalf("Elements() returned elements after deadline: %v", elements)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Elements() error = %v, want context deadline", err)
	}
	if appium.IsErrorCode(err, appium.CodeElementNotFound) {
		t.Fatalf("empty FindElements results must not invent a remote not-found error: %v", err)
	}
}

func TestElementsDeadlineKeepsLastNotFoundAcrossEmptyResults(t *testing.T) {
	notFound := &appium.Error{
		Code:      appium.CodeElementNotFound,
		Operation: "find_elements",
		Message:   "not present",
		Delivery:  appium.DeliveryAcknowledged,
	}
	finder := &stubFinder{
		elementsResults: []elementsResult{
			{err: notFound},
			{elements: []*appium.Element{}},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	elements, err := wait.Elements(
		ctx,
		2*time.Millisecond,
		finder,
		appium.ID("missing"),
	)
	if elements != nil {
		t.Fatalf("Elements() returned elements after deadline: %v", elements)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Elements() error = %v, want context deadline", err)
	}
	if !errors.Is(err, notFound) {
		t.Fatalf("Elements() error = %v, want last not-found error", err)
	}
}

func TestElementRejectsNilFinderWithoutCallingRemoteAPI(t *testing.T) {
	element, err := wait.Element(
		context.Background(),
		time.Millisecond,
		nil,
		appium.ID("login"),
	)
	if element != nil {
		t.Fatalf("Element() returned element for nil finder: %v", element)
	}
	if !appium.IsErrorCode(err, appium.CodeInvalidArgument) {
		t.Fatalf("Element() error = %v, want invalid argument", err)
	}
}

type elementResult struct {
	element *appium.Element
	err     error
}

type elementsResult struct {
	elements []*appium.Element
	err      error
}

type stubFinder struct {
	mu sync.Mutex

	elementResults  []elementResult
	elementsResults []elementsResult

	elementCalls  int
	elementsCalls int
	elementTimes  []time.Time
	elementLocs   []appium.Locator
}

func (f *stubFinder) Find(
	ctx context.Context,
	locator appium.Locator,
) (*appium.Element, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.elementCalls++
	f.elementTimes = append(f.elementTimes, time.Now())
	f.elementLocs = append(f.elementLocs, locator)

	if len(f.elementResults) == 0 {
		return nil, nil
	}
	index := f.elementCalls - 1
	if index >= len(f.elementResults) {
		index = len(f.elementResults) - 1
	}
	result := f.elementResults[index]
	return result.element, result.err
}

func (f *stubFinder) FindElements(
	ctx context.Context,
	locator appium.Locator,
) ([]*appium.Element, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.elementsCalls++
	if len(f.elementsResults) == 0 {
		return []*appium.Element{}, nil
	}
	index := f.elementsCalls - 1
	if index >= len(f.elementsResults) {
		index = len(f.elementsResults) - 1
	}
	result := f.elementsResults[index]
	return result.elements, result.err
}

func (f *stubFinder) elementCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.elementCalls
}

func (f *stubFinder) elementsCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.elementsCalls
}

func (f *stubFinder) elementCallTimes() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.elementTimes...)
}

func (f *stubFinder) locators() []appium.Locator {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]appium.Locator(nil), f.elementLocs...)
}
