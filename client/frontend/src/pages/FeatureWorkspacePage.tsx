import { ClockCircleOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

import { FeatureCard } from "../components/layout/FeatureCard";

type WorkspacePage = "tools";

const cardKeys: Record<WorkspacePage, readonly string[]> = {
  tools: ["ip", "unlock", "dns"],
};

interface FeatureWorkspacePageProps {
  page: WorkspacePage;
}

export function FeatureWorkspacePage({ page }: FeatureWorkspacePageProps) {
  const { t } = useTranslation();
  const base = `pages.${page}`;

  return (
    <div className="page-stack">
      <div className="feature-grid three-columns">
        {cardKeys[page].map((cardKey) => (
          <FeatureCard key={cardKey} title={t(`${base}.cards.${cardKey}`)}>
            <div className="planned-capability">
              <span className="planned-capability-icon">
                <ClockCircleOutlined />
              </span>
              <span>{t("common.planned")}</span>
            </div>
          </FeatureCard>
        ))}
      </div>
    </div>
  );
}
