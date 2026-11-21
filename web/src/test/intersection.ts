const observers = new Set<FakeIntersectionObserver>();

// jsdom has no layout, so a test says when observed elements come into view.
class FakeIntersectionObserver implements IntersectionObserver {
  readonly root = null;
  readonly rootMargin = "0px";
  readonly scrollMargin = "0px";
  readonly thresholds = [0];
  private readonly targets = new Set<Element>();
  private readonly callback: IntersectionObserverCallback;

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback;
    observers.add(this);
  }

  observe(target: Element) {
    this.targets.add(target);
  }

  unobserve(target: Element) {
    this.targets.delete(target);
  }

  disconnect() {
    this.targets.clear();
    observers.delete(this);
  }

  takeRecords() {
    return [];
  }

  enter() {
    const records = [...this.targets].map((target) => ({ target, isIntersecting: true }) as IntersectionObserverEntry);
    if (records.length > 0) this.callback(records, this);
  }
}

export function installIntersectionObserver() {
  window.IntersectionObserver = FakeIntersectionObserver;
}

export function scrollObservedIntoView() {
  for (const observer of [...observers]) observer.enter();
}
