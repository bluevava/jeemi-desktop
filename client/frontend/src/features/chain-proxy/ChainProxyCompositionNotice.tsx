import { useTranslation } from "react-i18next";
import type { ChainProxyComposition } from "../../types/chainProxy";

export function ChainProxyCompositionNotice({ composition }: { composition?: ChainProxyComposition }) {
  const { t } = useTranslation();
  if (!composition?.diagnostics?.length) return null;
  return <details className="chain-proxy-composition"><summary>{t("chainProxy.composition", { count: composition.generated })} · {t("chainProxy.diagnostics")}</summary>
    <ul>{composition.diagnostics.map((item, index) => <li key={index}>{t(`chainProxy.diagnostic.${item.code}`, { selector: item.selector, count: item.count })}</li>)}</ul>
  </details>;
}
