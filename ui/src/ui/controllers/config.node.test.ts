import { describe, expect, it } from "vitest";
import { needsInitialSetup } from "./config.ts";

describe("needsInitialSetup", () => {
  it("returns true when the config file does not exist", () => {
    expect(needsInitialSetup({ exists: false })).toBe(true);
  });

  it("returns true for an empty config object", () => {
    expect(
      needsInitialSetup({
        exists: true,
        valid: true,
        config: {},
      }),
    ).toBe(true);
  });

  it("returns false when the wizard has already completed", () => {
    expect(
      needsInitialSetup({
        exists: true,
        valid: true,
        config: {
          wizard: {
            lastRunAt: "2026-03-08T00:00:00Z",
          },
        },
      }),
    ).toBe(false);
  });

  it("returns false for a manually configured model setup without wizard metadata", () => {
    expect(
      needsInitialSetup({
        exists: true,
        valid: true,
        config: {
          agents: {
            defaults: {
              model: {
                primary: "openai/gpt-4o-mini",
              },
            },
          },
        },
      }),
    ).toBe(false);
  });

  it("does not auto-open for invalid existing config files", () => {
    expect(
      needsInitialSetup({
        exists: true,
        valid: false,
        config: {},
      }),
    ).toBe(false);
  });
});
