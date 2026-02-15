<template>
  <q-page padding>
    <div class="text-h5 text-weight-medium q-mb-lg">Settings</div>

    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading settings...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="loadError" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ loadError }}</div>
      <q-btn class="q-mt-md" color="primary" label="Retry" no-caps outline @click="fetchSettings" />
    </div>

    <!-- Content -->
    <template v-else>
      <q-card flat bordered class="cryo-card" style="max-width: 600px;">
        <q-card-section>
          <q-form @submit.prevent="onSubmit" class="q-gutter-md">
            <!-- Storage -->
            <div class="text-subtitle2 text-grey">Storage</div>
            <q-input
              v-model="form.defaultStorageClassName"
              label="Default Storage Class Name"
              hint="Leave empty for cluster default"
              outlined
            />

            <!-- Default Retention -->
            <div class="text-subtitle2 text-grey q-mt-md">Default Retention Policy</div>
            <div class="row q-col-gutter-sm">
              <div class="col-6">
                <q-input
                  v-model.number="form.defaultRetention.keepLast"
                  label="Keep Last"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
              <div class="col-6">
                <q-input
                  v-model.number="form.defaultRetention.keepDaily"
                  label="Keep Daily"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
              <div class="col-6">
                <q-input
                  v-model.number="form.defaultRetention.keepWeekly"
                  label="Keep Weekly"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
              <div class="col-6">
                <q-input
                  v-model.number="form.defaultRetention.keepMonthly"
                  label="Keep Monthly"
                  outlined
                  type="number"
                  :min="0"
                />
              </div>
            </div>

            <!-- Job Configuration -->
            <div class="text-subtitle2 text-grey q-mt-md">Job Configuration</div>
            <q-input
              v-model.number="form.jobTTLSeconds"
              label="Job TTL (seconds)"
              :hint="`Completed jobs are cleaned up after ${ttlDays}`"
              outlined
              type="number"
              :min="0"
            />

            <!-- Submit -->
            <div class="row justify-end q-mt-lg">
              <q-btn
                type="submit"
                label="Save Settings"
                color="primary"
                no-caps
                unelevated
                :loading="submitting"
              />
            </div>
          </q-form>
        </q-card-section>
      </q-card>

      <!-- Error banner -->
      <q-banner v-if="submitError" class="bg-negative text-white q-mt-md" rounded>
        <template v-slot:avatar>
          <q-icon name="error" />
        </template>
        {{ submitError }}
      </q-banner>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useQuasar } from 'quasar';
import settingsApi from 'src/api/settings';

const $q = useQuasar();

const loading = ref(true);
const loadError = ref('');
const submitting = ref(false);
const submitError = ref('');

const form = ref({
  defaultStorageClassName: '',
  defaultRetention: {
    keepLast: undefined as number | undefined,
    keepDaily: undefined as number | undefined,
    keepWeekly: undefined as number | undefined,
    keepMonthly: undefined as number | undefined,
  },
  jobTTLSeconds: 604800,
});

const ttlDays = computed(() => {
  const seconds = form.value.jobTTLSeconds || 0;
  const days = (seconds / 86400).toFixed(1);
  return `${days} days`;
});

async function fetchSettings() {
  loading.value = true;
  loadError.value = '';
  try {
    const s = await settingsApi.getSettings();
    form.value.defaultStorageClassName = s.defaultStorageClassName || '';
    form.value.jobTTLSeconds = s.jobTTLSeconds || 604800;
    if (s.defaultRetention) {
      form.value.defaultRetention.keepLast = s.defaultRetention.keepLast;
      form.value.defaultRetention.keepDaily = s.defaultRetention.keepDaily;
      form.value.defaultRetention.keepWeekly = s.defaultRetention.keepWeekly;
      form.value.defaultRetention.keepMonthly = s.defaultRetention.keepMonthly;
    }
  } catch (e) {
    loadError.value = 'Failed to load settings.';
    console.error('Failed to load settings:', e);
  } finally {
    loading.value = false;
  }
}

function buildRetention(): import('src/api/settings').RetentionPolicy | undefined {
  const r = form.value.defaultRetention;
  if (r.keepLast == null && r.keepDaily == null && r.keepWeekly == null && r.keepMonthly == null) {
    return undefined;
  }
  const result: import('src/api/settings').RetentionPolicy = {};
  if (r.keepLast) result.keepLast = r.keepLast;
  if (r.keepDaily) result.keepDaily = r.keepDaily;
  if (r.keepWeekly) result.keepWeekly = r.keepWeekly;
  if (r.keepMonthly) result.keepMonthly = r.keepMonthly;
  return result;
}

async function onSubmit() {
  submitting.value = true;
  submitError.value = '';
  try {
    const req: import('src/api/settings').UpdateSettingsRequest = {
      defaultStorageClassName: form.value.defaultStorageClassName,
      jobTTLSeconds: form.value.jobTTLSeconds,
    };
    const retention = buildRetention();
    if (retention) req.defaultRetention = retention;
    await settingsApi.updateSettings(req);
    $q.notify({
      type: 'positive',
      message: 'Settings saved successfully',
    });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    submitError.value = `Failed to save settings: ${msg}`;
    console.error('Failed to save settings:', e);
  } finally {
    submitting.value = false;
  }
}

onMounted(fetchSettings);
</script>
