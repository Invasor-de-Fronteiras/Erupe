export function randomArr<T>(arr: T[]) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export function randomHexColor() {
  return '#' + (((1 << 24) * Math.random()) | 0).toString(16);
}
