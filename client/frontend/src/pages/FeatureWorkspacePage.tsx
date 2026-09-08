import { ClockCircleOutlined } from "@ant-design/icons";
import { Collapse } from "antd";
import { useTranslation } from "react-i18next";

import { FeatureHelp } from "../components/help/FeatureHelp";
import { DNSQueryPanel } from "../features/tools/DNSQueryPanel";
import { useToolsPageStateField } from "../features/tools/ToolsPageStateContext";

type WorkspacePage = "tools";

const cardKeys: Record<WorkspacePage, readonly string[]> = {
  tools: ["dns", "ip", "unlock"],
};

interface FeatureWorkspacePageProps {
  page: WorkspacePage;
}

export function FeatureWorkspacePage({ page }: FeatureWorkspacePageProps) {
  const { t } = useTranslation();
  const [expandedPanels, setExpandedPanels] = useToolsPageStateField("expandedPanels");
  const base = `pages.${page}`;

  return (
    <div className="page-stack">
      <Collapse
        className="tools-panels"
        activeKey={expandedPanels}
        onChange={(keys) => setExpandedPanels(Array.isArray(keys) ? keys.map(String) : [String(keys)])}
        items={cardKeys[page].map((cardKey) => ({
          key: cardKey,
          label: t(`${base}.cards.${cardKey}`),
          extra: cardKey === "dns" ? (
            <span onClick={(event) => event.stopPropagation()}>
              <FeatureHelp compact topic="dnsQuery" />
            </span>
          ) : undefined,
          children: cardKey === "dns" ? <DNSQueryPanel /> : (
            <div className="planned-capability">
              <span className="planned-capability-icon">
                <ClockCircleOutlined />
              </span>
              <span>{t("common.planned")}</span>
            </div>
          ),
        }))}
      />
    </div>
  );
}
