import { useThing } from "@lib/hook";
import { Icon } from "@lib/icon";

export function Page() {
  const thing = useThing();
  return <Icon label={thing} />;
}
