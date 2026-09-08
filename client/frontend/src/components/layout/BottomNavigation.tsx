import { NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { visibleNavigationItems } from "../../app/navigation";
import { useNavigationGuard } from "../../app/navigationGuard/NavigationGuardContext";
import { useRuntimeStatus } from "../../app/runtime/RuntimeStatusContext";

export function BottomNavigation() {
  const { t } = useTranslation();
  const navigationGuard = useNavigationGuard();
  const { runtimePreferences } = useRuntimeStatus();
  const items = visibleNavigationItems(runtimePreferences?.logLevel);

  return (
    <nav aria-label={t("app.name")} className="bottom-navigation-wrap">
      <div className="bottom-navigation">
        {items.map((item) => (
          <NavLink
            className={({ isActive }) =>
              `bottom-navigation-item${isActive ? " active" : ""}`
            }
            key={item.path}
            onClick={(event) => {
              if (!navigationGuard.canLeave()) {
                event.preventDefault();
              }
            }}
            to={item.path}
          >
            <span className="bottom-navigation-icon">{item.icon}</span>
            <span>{t(item.labelKey)}</span>
          </NavLink>
        ))}
      </div>
    </nav>
  );
}
