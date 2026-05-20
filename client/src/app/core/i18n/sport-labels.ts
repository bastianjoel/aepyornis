import { _ } from '@ngx-translate/core';
import { getWorkoutTypeConfig } from '../types/workout-types';

const SPORT_LABELS: Record<string, string> = {
  auto: _('auto-detect'),
  unknown: _('unknown'),
};

const SPORT_SUBTYPE_LABELS: Record<string, string> = {
  virtual_activity: _('virtual activity'),
};

export const getSportLabel = (value?: string | null): string => {
  if (!value) {
    return '';
  }
  return SPORT_LABELS[value] ?? getWorkoutTypeConfig(value)?.name ?? '';
};

export const getSportSubtypeLabel = (value?: string | null): string => {
  if (!value) {
    return '';
  }
  return SPORT_SUBTYPE_LABELS[value] ?? value;
};
