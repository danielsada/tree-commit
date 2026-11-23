import { useState } from "react";
import { calculate, type Operation } from "./api";

export function App() {
  const [result, setResult] = useState<string>("");

  async function onSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const value = await calculate(
      form.get("operation") as Operation,
      Number(form.get("a")),
      Number(form.get("b")),
    );
    setResult(String(value));
  }

  return (
    <form onSubmit={onSubmit}>
      <input name="a" type="number" required />
      <select name="operation">
        <option value="add">+</option>
        <option value="subtract">-</option>
        <option value="multiply">*</option>
      </select>
      <input name="b" type="number" required />
      <button type="submit">=</button>
      <output>{result}</output>
    </form>
  );
}
