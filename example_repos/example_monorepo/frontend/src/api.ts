export type Operation = "add" | "subtract" | "multiply";

export async function calculate(operation: Operation, a: number, b: number): Promise<number> {
  const response = await fetch(`/api/${operation}?a=${a}&b=${b}`);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  const body: { result: number } = await response.json();
  return body.result;
}
