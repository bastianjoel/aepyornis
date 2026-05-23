import { _ } from '@ngx-translate/core';
import { getWorkoutTypeConfig, WORKOUT_SUB_TYPES } from '../types/workout-types';

const SPORT_LABELS: Record<string, string> = {
  auto: _('auto-detect'),
  unknown: _('unknown'),
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
  return WORKOUT_SUB_TYPES[value] ?? value;
};
