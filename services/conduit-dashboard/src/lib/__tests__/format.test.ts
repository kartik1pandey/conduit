import { describe, expect, it } from "vitest";
import { sumMoney } from "@/lib/format";

describe("sumMoney", () => {
  it("adds simple amounts without float drift", () => {
    expect(sumMoney(["10.00", "20.50", "5.25"])).toBe("35.75");
  });

  it("handles amounts that would drift under naive float addition", () => {
    // 0.1 + 0.2 famously != 0.3 in IEEE 754 float arithmetic — this is the
    // exact case this helper exists to avoid.
    expect(sumMoney(["0.10", "0.20"])).toBe("0.30");
  });

  it("returns 0.00 for an empty list", () => {
    expect(sumMoney([])).toBe("0.00");
  });

  it("handles a single amount", () => {
    expect(sumMoney(["99.99"])).toBe("99.99");
  });
});
