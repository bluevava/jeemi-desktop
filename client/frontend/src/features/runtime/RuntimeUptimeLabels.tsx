import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useWindowActivity } from "../../app/runtime/WindowActivityContext";
import { runtimeUptime, type RuntimeUptime } from "./runtimePresentation";

interface RuntimeUptimeLabelsProps {
  clientStartedAt: string;
  mihomoStartedAt: string | null;
}

export function RuntimeUptimeLabels({
  clientStartedAt,
  mihomoStartedAt,
}: RuntimeUptimeLabelsProps) {
  const { t } = useTranslation();
  const { isWindowVisible } = useWindowActivity();
  const [nowMilliseconds, setNowMilliseconds] = useState(() => Date.now());

  useEffect(() => {
    if (!isWindowVisible) return;

    setNowMilliseconds(Date.now());
    const timer = window.setInterval(() => {
      setNowMilliseconds(Date.now());
    }, 1000);
    return () => window.clearInterval(timer);
  }, [clientStartedAt, isWindowVisible, mihomoStartedAt]);

  const clientUptime = runtimeUptime(clientStartedAt, nowMilliseconds);
  const mihomoUptime = mihomoStartedAt
    ? runtimeUptime(mihomoStartedAt, nowMilliseconds)
    : null;
  if (!clientUptime && !mihomoUptime) return null;

  return (
    <div className="runtime-uptime-tags">
      {clientUptime ? (
        <UptimeTag
          kind="client"
          label={t("home.statusControl.uptime.client")}
          startedAt={clientStartedAt}
          uptime={clientUptime}
        />
      ) : null}
      {mihomoUptime && mihomoStartedAt ? (
        <UptimeTag
          kind="mihomo"
          label={t("home.statusControl.uptime.mihomo")}
          startedAt={mihomoStartedAt}
          uptime={mihomoUptime}
        />
      ) : null}
    </div>
  );
}

function UptimeTag({
  kind,
  label,
  startedAt,
  uptime,
}: {
  kind: "client" | "mihomo";
  label: string;
  startedAt: string;
  uptime: RuntimeUptime;
}) {
  const { t } = useTranslation();
  return (
    <time className={`runtime-uptime-tag ${kind}`} dateTime={startedAt}>
      <span>{label}</span>
      <strong>{uptime.value}</strong>
      <span>{t(`home.statusControl.uptime.units.${uptime.unit}`)}</span>
    </time>
  );
}
