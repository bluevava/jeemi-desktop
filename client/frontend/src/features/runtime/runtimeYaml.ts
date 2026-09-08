const yamlImplicitScalar = /^(?:null|~|true|false|yes|no|on|off|[-+]?(?:\d[\d_]*)(?:\.\d[\d_]*)?(?:e[-+]?\d+)?|\.nan|[-+]?\.inf)$/i;
const safePlainScalar = /^[A-Za-z0-9_./:+*?@%=,\-]+$/;

export function formatYamlStringSequence(values: string[]): string {
  if (values.length === 0) return "";
  return values.map((value) => `- ${formatYamlString(value)}`).join("\n");
}

function formatYamlString(value: string): string {
  const trimmed = value.trim();
  if (
    trimmed.length > 0 &&
    safePlainScalar.test(trimmed) &&
    !yamlImplicitScalar.test(trimmed)
  ) {
    return trimmed;
  }
  return JSON.stringify(trimmed);
}
