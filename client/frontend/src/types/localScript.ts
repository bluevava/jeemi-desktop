export interface LocalScriptSummary {
  id: string;
  name: string;
  description: string;
  revision: number;
  lineCount: number;
  sizeBytes: number;
  createdAt: string;
  updatedAt: string;
}

export interface LocalScript extends LocalScriptSummary {
  contents: string;
}

export interface LocalScriptState {
  directory: string;
  scripts: LocalScriptSummary[];
}

export interface SaveLocalScriptInput {
  id: string;
  name: string;
  description: string;
  contents: string;
}

export interface TestLocalScriptInput extends SaveLocalScriptInput {
  subscriptionId: string;
}

export interface LocalScriptTestResult {
  subscriptionId: string;
  subscriptionName: string;
  contents: string;
  coreValidated: boolean;
}
