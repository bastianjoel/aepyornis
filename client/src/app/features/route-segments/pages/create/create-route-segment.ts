import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  OnInit,
  signal,
} from '@angular/core';

import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Api } from '../../../../core/services/api';
import { AppIcon } from '../../../../core/components/app-icon/app-icon';
import { TranslatePipe } from '@ngx-translate/core';
import { WORKOUT_SUB_TYPES_BY_TYPE, WORKOUT_TYPES } from '../../../../core/types/workout-types';
import { getSportLabel, getSportSubtypeLabel } from '../../../../core/i18n/sport-labels';
import { RouteSegment } from '../../../../core/types/route-segment';

@Component({
  selector: 'app-create-route-segment',
  imports: [ReactiveFormsModule, AppIcon, TranslatePipe],
  templateUrl: './create-route-segment.html',
  styleUrl: './create-route-segment.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CreateRouteSegmentPage implements OnInit {
  private api = inject(Api);
  private router = inject(Router);
  private fb = inject(FormBuilder);

  public readonly selectedFiles = signal<File[]>([]);
  public readonly creating = signal(false);
  public readonly error = signal<string | null>(null);
  public readonly bidirectional = signal(false);
  public readonly circular = signal(false);

  public readonly availableTypes = signal<string[]>([]);
  public readonly subTypesByType = signal<Record<string, string[]>>({});
  public readonly selectedCategory = signal<string>('');

  public readonly availableSubTypes = computed(() => {
    const cat = this.selectedCategory();
    if (!cat) {
      return [];
    }
    return this.subTypesByType()[cat] || [];
  });

  public readonly sportLabel = getSportLabel;
  public readonly sportSubtypeLabel = getSportSubtypeLabel;

  public routeSegmentForm!: FormGroup;

  public readonly hasFiles = computed(() => this.selectedFiles().length > 0);
  public readonly fileCount = computed(() => this.selectedFiles().length);

  public ngOnInit(): void {
    this.routeSegmentForm = this.fb.group({
      notes: [''],
      category: [''],
      sub_category: [''],
      visibility: ['public'],
      difficulty: [''],
      description: [''],
    });

    this.loadFilterOptions();

    this.routeSegmentForm.get('category')?.valueChanges.subscribe((val) => {
      this.selectedCategory.set(val || '');
      const available = this.availableSubTypes();
      const currentSub = this.routeSegmentForm.get('sub_category')?.value;
      if (currentSub && available.length > 0 && !available.includes(currentSub)) {
        this.routeSegmentForm.patchValue({ sub_category: '' });
      }
    });
  }

  private async loadFilterOptions(): Promise<void> {
    const typesSet = new Set<string>();
    WORKOUT_TYPES.forEach((t) => {
      if (t.value !== 'all' && t.value !== 'auto') {
        typesSet.add(t.value);
      }
    });

    const subTypesMap: Record<string, string[]> = {};
    Object.entries(WORKOUT_SUB_TYPES_BY_TYPE).forEach(([type, subs]) => {
      subTypesMap[type] = [...subs];
    });

    try {
      const res = await firstValueFrom(this.api.getWorkoutFilterOptions());
      if (res?.results) {
        if (res.results.types?.length) {
          res.results.types.forEach((t) => typesSet.add(t));
        }
        if (res.results.sub_types_by_type) {
          Object.entries(res.results.sub_types_by_type).forEach(([type, subs]) => {
            const merged = new Set([...(subTypesMap[type] || []), ...subs]);
            subTypesMap[type] = Array.from(merged);
          });
        }
      }
    } catch (err) {
      console.error('Failed to load filter options:', err);
    }

    this.availableTypes.set(Array.from(typesSet));
    this.subTypesByType.set(subTypesMap);
  }

  public onFilesSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files) {
      this.selectedFiles.set(Array.from(input.files));
    }
  }

  public removeFile(index: number): void {
    const files = this.selectedFiles();
    files.splice(index, 1);
    this.selectedFiles.set([...files]);
  }

  public async createRouteSegment(): Promise<void> {
    if (this.creating()) {
      return;
    }

    const files = this.selectedFiles();
    if (files.length === 0) {
      this.error.set('Please select at least one file.');
      return;
    }

    this.creating.set(true);
    this.error.set(null);

    try {
      const formData = new FormData();
      files.forEach((file) => formData.append('file', file));

      const formVal = this.routeSegmentForm.value;
      const notesValue = String(formVal.notes || '').trim();
      if (notesValue.length > 0) {
        formData.append('notes', notesValue);
      }
      if (formVal.category) {
        formData.append('category', String(formVal.category).trim());
      }
      if (formVal.sub_category) {
        formData.append('sub_category', String(formVal.sub_category).trim());
      }
      if (formVal.visibility) {
        formData.append('visibility', String(formVal.visibility));
      }
      if (formVal.difficulty) {
        formData.append('difficulty', String(formVal.difficulty));
      }
      if (formVal.description) {
        formData.append('description', String(formVal.description).trim());
      }

      const response = await firstValueFrom(this.api.createRouteSegment(formData));
      const results = Array.isArray(response?.results) ? (response?.results as RouteSegment[]) : [];

      if (!results.length) {
        if (response?.errors?.length) {
          this.error.set(response.errors.join(' '));
        } else {
          this.error.set('Failed to create route segment. Please try again.');
        }
        return;
      }

      if (this.bidirectional() || this.circular()) {
        for (const segment of results) {
          await firstValueFrom(
            this.api.updateRouteSegment(segment.id, {
              name: segment.name,
              notes: segment.notes ?? notesValue,
              category: formVal.category,
              sub_category: formVal.sub_category,
              visibility: formVal.visibility,
              difficulty: formVal.difficulty,
              description: formVal.description,
              bidirectional: this.bidirectional(),
              circular: this.circular(),
            }),
          );
        }
      }

      if (results.length === 1) {
        this.router.navigate(['/route-segments', results[0].id]);
      } else {
        this.router.navigate(['/route-segments']);
      }
    } catch (err) {
      console.error('Failed to create route segment:', err);
      this.error.set('Failed to create route segment. Please try again.');
    } finally {
      this.creating.set(false);
    }
  }

  public goBack(): void {
    this.router.navigate(['/route-segments']);
  }
}
