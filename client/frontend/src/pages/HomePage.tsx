import { AboutJeemiCard } from "../features/runtime/AboutJeemiCard";
import { RuntimePreferencesCard } from "../features/runtime/RuntimePreferencesCard";
import { RuntimeStatusHero } from "../features/runtime/RuntimeStatusHero";
import { TrafficChartCard } from "../features/runtime/TrafficChartCard";
import { useRuntimePreferencesDraft } from "../features/runtime/useRuntimePreferencesDraft";

export function HomePage() {
  const preferences = useRuntimePreferencesDraft();

  return (
    <div className="page-stack">
      <RuntimeStatusHero preferences={preferences} />

      <div className="home-runtime-overview-grid">
        <TrafficChartCard />
        <AboutJeemiCard />
      </div>

      <RuntimePreferencesCard preferences={preferences} />
    </div>
  );
}
