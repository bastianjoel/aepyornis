/**
 * Route segment domain models
 */

export type RouteSegmentDifficulty = 'easy' | 'moderate' | 'difficult' | '';

export type RouteSegment = {
  id: number;
  profile_id: number;
  profile_name?: string;
  name: string;
  notes?: string;
  category?: string;
  sub_category?: string;
  visibility: 'public' | 'followers' | '' | 'private';
  description?: string;
  difficulty?: RouteSegmentDifficulty;
  filename: string;
  total_distance: number;
  min_elevation: number;
  max_elevation: number;
  total_up: number;
  total_down: number;
  bidirectional: boolean;
  circular: boolean;
  match_count: number;
  created_at: string;
  updated_at: string;
};

export type MapPoint = {
  lat: number;
  lng: number;
  elevation: number;
  total_distance: number;
};

export type RouteSegmentMatch = {
  workout_id: number;
  workout_name: string;
  user_id: number;
  user_name: string;
  distance: number;
  duration: number;
  average_speed: number;
};

export type RouteSegmentDetail = {
  points: MapPoint[];
  matches: RouteSegmentMatch[];
  center: {
    lat: number;
    lng: number;
  };
  address_string: string;
} & RouteSegment;
