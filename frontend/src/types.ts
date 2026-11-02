export type Worker = {
  id: string;
  name: string;
  title: string;
  organization: string;
};

export type Asset = {
  id: string;
  name: string;
  campus: string;
  building: string;
  section: string;
  level: string;
  wing: string;
  floor: string;
  space: string;
  priority: string;
  state: string;
  tierId?: string;
  lastLaunchId?: string;
  businessOwner?: string;
  businessManager?: string;
};

export type Launch = {
  id: string;
  assetId: string;
  tierId?: string;
  state: string;
  createdAt: string;
  updatedAt: string;
};

export type Tier = {
  id: string;
  name: string;
  title: string;
  description: string;
  fromPoint: number;
  toPoint: number;
};

export type QuestionCategory = {
  id: string;
  title: string;
  text: string;
  type: string;
  createdAt: string;
};

export type Question = {
  id: string;
  categoryId: string;
  text: string;
  allowMultipleAnswers: boolean;
  displayOrder: number;
};

export type PredefinedAnswer = {
  id: string;
  questionId: string;
  text: string;
  points: number;
  displayOrder: number;
};

export type UserAnswerDetail = {
  questionId: string;
  selections: Array<{
    predefinedAnswerId: string;
  }>;
};

export type Baseline = {
  id: string;
  state: string;
  createdAt: string;
};

export type AssetRequest = {
  name: string;
  campus: string;
  building: string;
  floor: string;
  space: string;
  priority: string;
  businessOwnerId: string;
  businessManagerId: string;
};
