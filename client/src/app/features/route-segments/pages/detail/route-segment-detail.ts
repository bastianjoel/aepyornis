import { ChangeDetectionStrategy, Component, inject, OnInit, signal } from '@angular/core';

import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../../../core/services/api';
import { RouteSegmentDetail, RouteSegmentMatch } from '../../../../core/types/route-segment';
import { Profile } from '../../../../core/types/user';
import { AppIcon } from '../../../../core/components/app-icon/app-icon';
import { RouteSegmentActionsComponent } from '../../../route-segments/components/route-segment-actions/route-segment-actions';
import { TranslatePipe } from '@ngx-translate/core';
import { RouteSegmentMapComponent } from '../../components/route-segment-map/route-segment-map';
import { getSportLabel, getSportSubtypeLabel } from '../../../../core/i18n/sport-labels';
import { getMetricDef } from '../../../../core/config/metrics';

@Component({
  selector: 'app-route-segment-detail',
  imports: [
    RouterLink,
    AppIcon,
    RouteSegmentActionsComponent,
    TranslatePipe,
    RouteSegmentMapComponent,
  ],
  templateUrl: './route-segment-detail.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RouteSegmentDetailPage implements OnInit {
  public readonly getMetricDef = getMetricDef;
  public readonly sportLabel = getSportLabel;
  public readonly sportSubtypeLabel = getSportSubtypeLabel;
  private api = inject(Api);
  private route = inject(ActivatedRoute);
  private router = inject(Router);

  public readonly routeSegment = signal<RouteSegmentDetail | null>(null);
  public readonly loading = signal(true);
  public readonly error = signal<string | null>(null);

  // Likes state
  public readonly likeCount = signal(0);
  public readonly hasLiked = signal(false);
  public readonly isLiking = signal(false);
  public readonly likersModalOpen = signal(false);
  public readonly likersList = signal<Profile[]>([]);

  // Matches state
  public readonly matches = signal<RouteSegmentMatch[]>([]);
  public readonly matchesLoading = signal(false);
  public readonly matchesPage = signal(1);
  public readonly matchesPerPage = signal(10);
  public readonly matchesTotalCount = signal(0);
  public readonly matchesTotalPages = signal(1);
  public readonly matchesSort = signal<'best' | 'newest'>('best');

  public ngOnInit(): void {
    this.route.params.subscribe((params) => {
      const id = parseInt(params['id']);
      if (id) {
        this.loadRouteSegment(id);
      }
    });
  }

  public async loadRouteSegment(id: number): Promise<void> {
    this.loading.set(true);
    this.error.set(null);

    try {
      const response = await firstValueFrom(this.api.getRouteSegment(id));

      if (response && response.results) {
        const seg = response.results;
        this.routeSegment.set(seg);
        this.likeCount.set(seg.like_count || 0);
        this.hasLiked.set(!!seg.has_liked);

        this.loadMatches(id, 1, this.matchesSort());
      }
    } catch (err) {
      console.error('Failed to load route segment:', err);
      this.error.set('Failed to load route segment. Please try again.');
    } finally {
      this.loading.set(false);
    }
  }

  public async loadMatches(id: number, page: number, sort: 'best' | 'newest'): Promise<void> {
    this.matchesLoading.set(true);
    this.matchesPage.set(page);

    try {
      const res = await firstValueFrom(
        this.api.getRouteSegmentMatches(id, { page, per_page: this.matchesPerPage(), sort }),
      );
      if (res && res.results) {
        this.matches.set(res.results);
        this.matchesTotalCount.set(res.total_count || res.results.length);
        this.matchesTotalPages.set(res.total_pages || 1);
      }
    } catch (err) {
      console.error('Failed to load matches:', err);
    } finally {
      this.matchesLoading.set(false);
    }
  }

  public changeMatchesSort(sort: 'best' | 'newest'): void {
    if (this.matchesSort() === sort) {
      return;
    }
    this.matchesSort.set(sort);
    const seg = this.routeSegment();
    if (seg) {
      this.loadMatches(seg.id, 1, sort);
    }
  }

  public changeMatchesPage(page: number): void {
    if (page < 1 || page > this.matchesTotalPages()) {
      return;
    }
    const seg = this.routeSegment();
    if (seg) {
      this.loadMatches(seg.id, page, this.matchesSort());
    }
  }

  public toggleLike(): void {
    const seg = this.routeSegment();
    if (!seg || this.isLiking()) {
      return;
    }

    this.isLiking.set(true);
    const id = seg.id;

    if (this.hasLiked()) {
      this.api.unlikeRouteSegment(id).subscribe({
        next: (res) => {
          this.hasLiked.set(false);
          this.likeCount.set(res.results.like_count);
          this.isLiking.set(false);
        },
        error: () => this.isLiking.set(false),
      });
    } else {
      this.api.likeRouteSegment(id).subscribe({
        next: (res) => {
          this.hasLiked.set(true);
          this.likeCount.set(res.results.like_count);
          this.isLiking.set(false);
        },
        error: () => this.isLiking.set(false),
      });
    }
  }

  public openLikersModal(): void {
    const seg = this.routeSegment();
    if (!seg) {
      return;
    }
    this.api.getRouteSegmentLikers(seg.id).subscribe({
      next: (res) => {
        this.likersList.set(res.results || []);
        this.likersModalOpen.set(true);
      },
      error: (err) => console.error('Failed to fetch likers', err),
    });
  }

  public closeLikersModal(): void {
    this.likersModalOpen.set(false);
  }

  public onRouteSegmentUpdated(): void {
    const id = this.route.snapshot.params['id'];
    if (id) {
      this.loadRouteSegment(parseInt(id));
    }
  }

  public onRouteSegmentDeleted(): void {
    // Navigation handled by actions component
  }

  public formatDate(dateString?: string): string {
    if (!dateString) {
      return '';
    }
    return new Date(dateString).toLocaleDateString();
  }

  public formatDistance(distance: number): string {
    return (distance / 1000).toFixed(2);
  }

  public formatDuration(seconds: number): string {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;

    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${secs.toString().padStart(2, '0')}`;
  }

  public formatSpeed(speedMs: number): string {
    return (speedMs * 3.6).toFixed(2);
  }

  public formatTempo(speedMs: number): string {
    if (speedMs === 0) {
      return '-';
    }
    const tempoSecondsPerKm = 1000 / speedMs;
    const minutes = Math.floor(tempoSecondsPerKm / 60);
    const seconds = Math.floor(tempoSecondsPerKm % 60);
    return `${minutes}:${seconds.toString().padStart(2, '0')}`;
  }

  public onDownloadFile(): void {
    const segment = this.routeSegment();
    if (!segment) {
      return;
    }

    this.api.downloadRouteSegment(segment.id).subscribe({
      next: (blob) => {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = segment.filename || `route_segment_${segment.id}.gpx`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
      },
      error: (err) => {
        console.error('Failed to download route segment file:', err);
      },
    });
  }

  public goBack(): void {
    this.router.navigate(['/route-segments']);
  }
}
