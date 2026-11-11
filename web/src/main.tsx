import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/app";
import { createApp } from "@/create-app";
import { UpdatePrompt } from "@/components/update-prompt";

import "./index.css";

const app = createApp();

const root = document.getElementById("root");
if (!root) throw new Error("index.html has no #root element");

createRoot(root).render(
  <StrictMode>
    <App app={app} />
    <UpdatePrompt />
  </StrictMode>,
);
