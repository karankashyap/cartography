import { test, expect } from "@playwright/test";
import path from "path";

const SAMPLE_CSV = path.resolve(__dirname, "../../../sample-data/shopify_orders.csv");

test("import shopify CSV → dashboard shows metrics", async ({ page }) => {
  await page.goto("http://localhost:3000");
  await page.getByRole("button", { name: /import/i }).click();
  await page.locator('input[type="file"]').setInputFiles(SAMPLE_CSV);
  await page.getByRole("button", { name: /shopify/i }).click();
  await page.getByRole("button", { name: /start import/i }).click();
  await expect(page.getByText(/import complete/i)).toBeVisible({ timeout: 90_000 });
  await page.goto("http://localhost:3000/dashboard");
  await expect(page.getByTestId("metric-revenue")).toBeVisible();
  const revenue = await page.getByTestId("metric-revenue").textContent();
  expect(Number(revenue?.replace(/[^0-9.]/g, ""))).toBeGreaterThan(0);
});

test("chat: safe query returns results", async ({ page }) => {
  await page.goto("http://localhost:3000/chat");
  await page.getByRole("textbox").fill("What are the top 5 products by revenue?");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("table")).toBeVisible({ timeout: 30_000 });
});

test("chat: write query is blocked", async ({ page }) => {
  await page.goto("http://localhost:3000/chat");
  await page.getByRole("textbox").fill("Delete all orders");
  await page.keyboard.press("Enter");
  await expect(page.getByText(/blocked|not allowed|read.only/i)).toBeVisible({ timeout: 15_000 });
});

test("content studio: generates description", async ({ page }) => {
  await page.goto("http://localhost:3000/content");
  await page.getByRole("checkbox").first().check();
  await page.getByRole("button", { name: /description/i }).click();
  await expect(page.getByRole("textbox")).not.toBeEmpty({ timeout: 30_000 });
});
