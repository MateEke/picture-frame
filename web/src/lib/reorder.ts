export function arrayMove<T>(items: readonly T[], from: number, to: number): T[] {
	const next = [...items];
	const [moved] = next.splice(from, 1);
	next.splice(to, 0, moved);
	return next;
}

export function moveUp<T>(items: readonly T[], index: number): T[] {
	if (index <= 0) return [...items];
	return arrayMove(items, index, index - 1);
}

export function moveDown<T>(items: readonly T[], index: number): T[] {
	return arrayMove(items, index, index + 1);
}

export function moveToStart<T>(items: readonly T[], index: number): T[] {
	return arrayMove(items, index, 0);
}

export function moveToEnd<T>(items: readonly T[], index: number): T[] {
	return arrayMove(items, index, items.length - 1);
}
