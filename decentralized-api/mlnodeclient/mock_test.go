package mlnodeclient

import (
	"context"
	"errors"
	"testing"
)

func TestMockClient_GetGPUDevices(t *testing.T) {
	ctx := context.Background()

	t.Run("returns configured devices", func(t *testing.T) {
		mock := NewMockClient()
		mock.GPUDevices = []GPUDevice{
			{Index: 0, Name: "NVIDIA A100", IsAvailable: true},
			{Index: 1, Name: "NVIDIA V100", IsAvailable: true},
		}

		resp, err := mock.GetGPUDevices(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected count 2, got %d", resp.Count)
		}
		if len(resp.Devices) != 2 {
			t.Errorf("expected 2 devices, got %d", len(resp.Devices))
		}
		if mock.GetGPUDevicesCalled != 1 {
			t.Errorf("expected GetGPUDevicesCalled to be 1, got %d", mock.GetGPUDevicesCalled)
		}
	})

	t.Run("returns error when configured", func(t *testing.T) {
		mock := NewMockClient()
		mock.GetGPUDevicesError = errors.New("test error")

		_, err := mock.GetGPUDevices(ctx)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "test error" {
			t.Errorf("expected error 'test error', got %v", err)
		}
	})

	t.Run("returns empty list by default", func(t *testing.T) {
		mock := NewMockClient()

		resp, err := mock.GetGPUDevices(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Count != 0 {
			t.Errorf("expected count 0, got %d", resp.Count)
		}
	})
}

func TestMockClient_GetGPUDriver(t *testing.T) {
	ctx := context.Background()

	t.Run("returns default driver info", func(t *testing.T) {
		mock := NewMockClient()

		resp, err := mock.GetGPUDriver(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.DriverVersion != "535.104.05" {
			t.Errorf("expected driver version 535.104.05, got %s", resp.DriverVersion)
		}
		if mock.GetGPUDriverCalled != 1 {
			t.Errorf("expected GetGPUDriverCalled to be 1, got %d", mock.GetGPUDriverCalled)
		}
	})

	t.Run("returns configured driver info", func(t *testing.T) {
		mock := NewMockClient()
		mock.DriverInfo = &DriverInfo{
			DriverVersion:     "550.54.15",
			CudaDriverVersion: "12.4",
			NvmlVersion:       "12.550.54",
		}

		resp, err := mock.GetGPUDriver(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.DriverVersion != "550.54.15" {
			t.Errorf("expected driver version 550.54.15, got %s", resp.DriverVersion)
		}
	})

	t.Run("returns error when configured", func(t *testing.T) {
		mock := NewMockClient()
		mock.GetGPUDriverError = errors.New("driver error")

		_, err := mock.GetGPUDriver(ctx)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestMockClient_CheckModelStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not found for unknown model", func(t *testing.T) {
		mock := NewMockClient()
		model := Model{HfRepo: "test/model", HfCommit: nil}

		resp, err := mock.CheckModelStatus(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != ModelStatusNotFound {
			t.Errorf("expected status NOT_FOUND, got %s", resp.Status)
		}
		if mock.CheckModelStatusCalled != 1 {
			t.Errorf("expected CheckModelStatusCalled to be 1, got %d", mock.CheckModelStatusCalled)
		}
		if mock.LastModelStatusCheck == nil {
			t.Fatal("expected LastModelStatusCheck to be set")
		}
		if mock.LastModelStatusCheck.HfRepo != "test/model" {
			t.Errorf("expected repo test/model, got %s", mock.LastModelStatusCheck.HfRepo)
		}
	})

	t.Run("returns downloading status", func(t *testing.T) {
		mock := NewMockClient()
		model := Model{HfRepo: "test/model", HfCommit: nil}

		mock.DownloadingModels["test/model:latest"] = &DownloadProgress{
			StartTime:      1728565234.0,
			ElapsedSeconds: 10.5,
		}

		resp, err := mock.CheckModelStatus(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != ModelStatusDownloading {
			t.Errorf("expected status DOWNLOADING, got %s", resp.Status)
		}
		if resp.Progress == nil {
			t.Fatal("expected progress info")
		}
	})

	t.Run("returns cached model status", func(t *testing.T) {
		mock := NewMockClient()
		model := Model{HfRepo: "test/model", HfCommit: nil}

		mock.CachedModels["test/model:latest"] = ModelListItem{
			Model:  model,
			Status: ModelStatusDownloaded,
		}

		resp, err := mock.CheckModelStatus(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != ModelStatusDownloaded {
			t.Errorf("expected status DOWNLOADED, got %s", resp.Status)
		}
	})
}

func TestMockClient_DownloadModel(t *testing.T) {
	ctx := context.Background()

	t.Run("starts download", func(t *testing.T) {
		mock := NewMockClient()
		model := Model{HfRepo: "test/model", HfCommit: nil}

		resp, err := mock.DownloadModel(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != ModelStatusDownloading {
			t.Errorf("expected status DOWNLOADING, got %s", resp.Status)
		}
		if resp.TaskId != "test/model:latest" {
			t.Errorf("expected task_id test/model:latest, got %s", resp.TaskId)
		}
		if mock.DownloadModelCalled != 1 {
			t.Errorf("expected DownloadModelCalled to be 1, got %d", mock.DownloadModelCalled)
		}
		if _, exists := mock.DownloadingModels["test/model:latest"]; !exists {
			t.Error("expected model to be added to DownloadingModels")
		}
	})

	t.Run("handles specific commit", func(t *testing.T) {
		mock := NewMockClient()
		commit := "abc123"
		model := Model{HfRepo: "test/model", HfCommit: &commit}

		resp, err := mock.DownloadModel(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.TaskId != "test/model:abc123" {
			t.Errorf("expected task_id test/model:abc123, got %s", resp.TaskId)
		}
	})

	t.Run("returns error when configured", func(t *testing.T) {
		mock := NewMockClient()
		mock.DownloadModelError = errors.New("download error")
		model := Model{HfRepo: "test/model"}

		_, err := mock.DownloadModel(ctx, model)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestMockClient_DeleteModel(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes cached model", func(t *testing.T) {
		mock := NewMockClient()
		model := Model{HfRepo: "test/model", HfCommit: nil}

		mock.CachedModels["test/model:latest"] = ModelListItem{
			Model:  model,
			Status: ModelStatusDownloaded,
		}

		resp, err := mock.DeleteModel(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != "deleted" {
			t.Errorf("expected status deleted, got %s", resp.Status)
		}
		if _, exists := mock.CachedModels["test/model:latest"]; exists {
			t.Error("expected model to be removed from cache")
		}
		if mock.DeleteModelCalled != 1 {
			t.Errorf("expected DeleteModelCalled to be 1, got %d", mock.DeleteModelCalled)
		}
	})

	t.Run("cancels downloading model", func(t *testing.T) {
		mock := NewMockClient()
		model := Model{HfRepo: "test/model", HfCommit: nil}

		mock.DownloadingModels["test/model:latest"] = &DownloadProgress{
			StartTime:      1728565234.0,
			ElapsedSeconds: 10.0,
		}

		resp, err := mock.DeleteModel(ctx, model)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Status != "cancelled" {
			t.Errorf("expected status cancelled, got %s", resp.Status)
		}
		if _, exists := mock.DownloadingModels["test/model:latest"]; exists {
			t.Error("expected model to be removed from downloading models")
		}
	})
}

func TestMockClient_ListModels(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty list by default", func(t *testing.T) {
		mock := NewMockClient()

		resp, err := mock.ListModels(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Models) != 0 {
			t.Errorf("expected 0 models, got %d", len(resp.Models))
		}
		if mock.ListModelsCalled != 1 {
			t.Errorf("expected ListModelsCalled to be 1, got %d", mock.ListModelsCalled)
		}
	})

	t.Run("returns cached models", func(t *testing.T) {
		mock := NewMockClient()
		model1 := Model{HfRepo: "test/model1", HfCommit: nil}
		model2 := Model{HfRepo: "test/model2", HfCommit: nil}

		mock.CachedModels["test/model1:latest"] = ModelListItem{
			Model:  model1,
			Status: ModelStatusDownloaded,
		}
		mock.CachedModels["test/model2:latest"] = ModelListItem{
			Model:  model2,
			Status: ModelStatusPartial,
		}

		resp, err := mock.ListModels(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Models) != 2 {
			t.Errorf("expected 2 models, got %d", len(resp.Models))
		}
	})
}

func TestMockClient_GetDiskSpace(t *testing.T) {
	ctx := context.Background()

	t.Run("returns default disk space", func(t *testing.T) {
		mock := NewMockClient()

		resp, err := mock.GetDiskSpace(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.CacheSizeGB != 13.0 {
			t.Errorf("expected cache size 13.0, got %f", resp.CacheSizeGB)
		}
		if resp.AvailableGB != 465.66 {
			t.Errorf("expected available 465.66, got %f", resp.AvailableGB)
		}
		if mock.GetDiskSpaceCalled != 1 {
			t.Errorf("expected GetDiskSpaceCalled to be 1, got %d", mock.GetDiskSpaceCalled)
		}
	})

	t.Run("returns configured disk space", func(t *testing.T) {
		mock := NewMockClient()
		mock.DiskSpace = &DiskSpaceInfo{
			CacheSizeGB: 50.0,
			AvailableGB: 100.0,
			CachePath:   "/custom/path",
		}

		resp, err := mock.GetDiskSpace(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.CacheSizeGB != 50.0 {
			t.Errorf("expected cache size 50.0, got %f", resp.CacheSizeGB)
		}
	})
}

func TestGetModelKey(t *testing.T) {
	t.Run("without commit", func(t *testing.T) {
		model := Model{HfRepo: "test/model", HfCommit: nil}
		key := getModelKey(model)
		if key != "test/model:latest" {
			t.Errorf("expected key test/model:latest, got %s", key)
		}
	})

	t.Run("with commit", func(t *testing.T) {
		commit := "abc123"
		model := Model{HfRepo: "test/model", HfCommit: &commit}
		key := getModelKey(model)
		if key != "test/model:abc123" {
			t.Errorf("expected key test/model:abc123, got %s", key)
		}
	})

	t.Run("with empty commit string", func(t *testing.T) {
		commit := ""
		model := Model{HfRepo: "test/model", HfCommit: &commit}
		key := getModelKey(model)
		if key != "test/model:latest" {
			t.Errorf("expected key test/model:latest, got %s", key)
		}
	})
}
