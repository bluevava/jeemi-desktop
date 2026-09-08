import type { PropsWithChildren, ReactNode } from "react";
import { Card } from "antd";

import { FeatureHelp, type HelpTopic } from "../help/FeatureHelp";

interface FeatureCardProps extends PropsWithChildren {
  title: string;
  titleAddon?: ReactNode;
  headerAction?: ReactNode;
  helpTopic?: HelpTopic;
  className?: string;
}

export function FeatureCard({
  title,
  titleAddon,
  headerAction,
  helpTopic,
  className = "",
  children,
}: FeatureCardProps) {
  return (
    <Card className={`feature-card ${className}`} variant="borderless">
      <div className="feature-card-heading">
        <div>
          <div className="feature-card-title-line">
            <h2>{title}</h2>
            {helpTopic ? <FeatureHelp compact topic={helpTopic} /> : null}
            {titleAddon}
          </div>
        </div>
        {headerAction ? (
          <div className="feature-card-heading-actions">
            {headerAction}
          </div>
        ) : null}
      </div>
      {children}
    </Card>
  );
}
