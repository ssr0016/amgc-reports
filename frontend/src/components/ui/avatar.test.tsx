import { fireEvent, render } from "@testing-library/react";
import { expect, it } from "vitest";
import { Avatar } from "./avatar";

it("shows initials without a picture and falls back when a picture fails", () => {
  const { container, rerender } = render(<Avatar name="Ana Cruz" />);
  expect(container).toHaveTextContent("AC");
  expect(container.querySelector("img")).toBeNull();
  rerender(<Avatar name="Ana Cruz" src="https://cdn.example/a?v=1" />);
  const img = container.querySelector("img")!;
  expect(img).toHaveAttribute("src", "https://cdn.example/a?v=1");
  expect(img).toHaveAttribute("alt", "");
  fireEvent.error(img);
  expect(container.querySelector("img")).toBeNull();
  expect(container).toHaveTextContent("AC");
  rerender(<Avatar name="Ana Cruz" src="https://cdn.example/a?v=2" />);
  expect(container.querySelector("img")).toHaveAttribute("src", "https://cdn.example/a?v=2");
});
