export function readOptionValue(
  current: string,
  next: string | undefined,
  name: string,
): { consumedNext: boolean; value: string | undefined } {
  if (current === name) {
    if (next === undefined) {
      return { consumedNext: false, value: undefined };
    }
    return { consumedNext: true, value: next };
  }

  const prefix = `${name}=`;
  if (current.startsWith(prefix)) {
    return { consumedNext: false, value: current.slice(prefix.length) };
  }

  return { consumedNext: false, value: undefined };
}
