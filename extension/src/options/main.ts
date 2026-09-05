import { createApp } from "vue";
import App from "./App.vue";
import icons from "@/assets/icons.svg?raw";
import "@/styles/tokens.css";
import "@/styles/themes.css";
import "@/styles/components.css";
import "./options.css";

const sprite = document.createElement("div");
sprite.style.display = "none";
sprite.innerHTML = icons;
document.body.prepend(sprite);

createApp(App).mount("#app");
