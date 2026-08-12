package sftpclient

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/novassh/novassh/pkg/models"
	"github.com/novassh/novassh/pkg/sshclient"
	"github.com/pkg/sftp"
)

type ClientManager struct {
	sshManager *sshclient.Manager
}

func NewClientManager(sm *sshclient.Manager) *ClientManager {
	return &ClientManager{
		sshManager: sm,
	}
}

func (cm *ClientManager) getSFTP(s *models.Server) (*sftp.Client, error) {
	sshConn, err := cm.sshManager.Connect(s)
	if err != nil {
		return nil, err
	}
	client, err := sftp.NewClient(sshConn)
	if err != nil {
		return nil, fmt.Errorf("خطا در باز کردن نشست SFTP: %v", err)
	}
	return client, nil
}

// ListFiles returns current path, parent path, and sorted items in remote path
func (cm *ClientManager) ListFiles(s *models.Server, dirPath string) (*models.SFTPListResult, error) {
	client, err := cm.getSFTP(s)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if dirPath == "" || dirPath == "." {
		if wd, errWd := client.Getwd(); errWd == nil && wd != "" {
			dirPath = wd
		} else {
			dirPath = "."
		}
	}

	realPath, err := client.RealPath(dirPath)
	if err == nil && realPath != "" {
		dirPath = realPath
	}

	entries, err := client.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("خطا در خواندن پوشه '%s': %v", dirPath, err)
	}

	var files []models.SFTPFile
	for _, fi := range entries {
		fullPath := pathJoinSFTP(dirPath, fi.Name())
		files = append(files, models.SFTPFile{
			Name:        fi.Name(),
			Path:        fullPath,
			Size:        fi.Size(),
			SizeStr:     formatSize(fi.Size(), fi.IsDir()),
			IsDir:       fi.IsDir(),
			ModTime:     fi.ModTime().Format("2006-01-02 15:04"),
			Permissions: fi.Mode().String(),
		})
	}

	// Sort: Directories first, then by filename ascending
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	parentPath := path.Dir(dirPath)
	if parentPath == "" {
		parentPath = "/"
	}

	return &models.SFTPListResult{
		CurrentPath: dirPath,
		ParentPath:  parentPath,
		Files:       files,
	}, nil
}

func (cm *ClientManager) UploadFile(s *models.Server, remotePath string, reader io.Reader) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	parent := path.Dir(remotePath)
	_ = client.MkdirAll(parent)

	f, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("خطا در ایجاد فایل روی سرور: %v", err)
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	if err != nil {
		return fmt.Errorf("خطا در آپلود اطلاعات فایل: %v", err)
	}
	return nil
}

func (cm *ClientManager) DownloadFile(s *models.Server, remotePath string, writer io.Writer) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	f, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("فایل روی سرور یافت نشد یا غیرقابل دسترسی است: %v", err)
	}
	defer f.Close()

	_, err = io.Copy(writer, f)
	return err
}

func (cm *ClientManager) CreateDir(s *models.Server, remotePath string) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("خطا در ساخت پوشه جدید: %v", err)
	}
	return nil
}

func (cm *ClientManager) CreateFile(s *models.Server, remotePath string) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	f, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("خطا در ساخت فایل: %v", err)
	}
	f.Close()
	return nil
}

func (cm *ClientManager) Delete(s *models.Server, remotePath string) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	fi, err := client.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("مسیر یافت نشد: %v", err)
	}

	if fi.IsDir() {
		_, errRm := cm.sshManager.RunCommand(s, fmt.Sprintf("rm -rf %q", remotePath))
		return errRm
	}
	return client.Remove(remotePath)
}

func (cm *ClientManager) Rename(s *models.Server, oldPath string, newPath string) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Rename(oldPath, newPath)
}

func (cm *ClientManager) ReadFileContent(s *models.Server, remotePath string) (string, error) {
	client, err := cm.getSFTP(s)
	if err != nil {
		return "", err
	}
	defer client.Close()

	f, err := client.Open(remotePath)
	if err != nil {
		return "", fmt.Errorf("خطا در خواندن فایل: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 2*1024*1024)) // 2MB limit for preview
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (cm *ClientManager) WriteFileContent(s *models.Server, remotePath string, content string) error {
	client, err := cm.getSFTP(s)
	if err != nil {
		return err
	}
	defer client.Close()

	f, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("خطا در نوشتن فایل: %v", err)
	}
	defer f.Close()

	_, err = f.Write([]byte(content))
	return err
}

func pathJoinSFTP(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" {
		return "/" + name
	}
	return dir + "/" + name
}

func formatSize(size int64, isDir bool) string {
	if isDir {
		return "-"
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024.0)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024.0*1024.0))
	}
	return fmt.Sprintf("%.2f GB", float64(size)/(1024.0*1024.0*1024.0))
}
