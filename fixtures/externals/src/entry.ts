import { acme } from "acme";
import { leftpad } from "leftpad";
import { local } from "./local.ts";

export const x = () => acme() + leftpad("1") + local();
