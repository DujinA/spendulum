import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { UpdatePrompt } from "@/components/update-prompt";
import { createAppRouter } from "@/router";

import "./index.css";

const router = createAppRouter();

const root = document.getElementById("root");
if (!root) throw new Error("index.html has no #root element");

createRoot(root).render(
  <StrictMode>
    <RouterProvider router={router} />
    <UpdatePrompt />
  </StrictMode>,
);
