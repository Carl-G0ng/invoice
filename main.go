package main

import (
	"archive/zip"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
	"time"

	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultApiKey  = "384e92997199140a8eda3cc184ecf29ecab602d9230504e6224024f0d4315c80"
	defaultMembers = "Carl.Gong, Xuhui.Shi, Makiyo.Jiang, Sunny Tang"
)

func init() {
	apiKey := defaultApiKey
	// 获取命令行参数
	args := os.Args[1:]
	if len(args) == 1 {
		apiKey = args[0]
	}

	// To get your free API key for metered license, sign up on: https://cloud.unidoc.io
	// Make sure to be using UniPDF v3.19.1 or newer for Metered API key support.
	err := license.SetMeteredKey(apiKey)
	if err != nil {
		fmt.Printf("ERROR: Failed to set metered key: %v\n", err)
		fmt.Printf("Make sure to get a valid key from https://cloud.unidoc.io\n")
		panic(err)
	}
	lk := license.GetLicenseKey()
	if lk == nil {
		fmt.Printf("Failed retrieving license key")
		return
	}
	//fmt.Printf("License: %s\n", lk.ToString())

	// GetMeteredState freshly checks the state, contacting the licensing server.
	state, err := license.GetMeteredState()
	if err != nil {
		fmt.Printf("ERROR getting metered state: %+v\n", err)
		panic(err)
	}
	//fmt.Printf("State: %+v\n", state)
	if state.OK {
		//fmt.Printf("State is OK\n")
	} else {
		fmt.Printf("State is not OK\n")
	}
}

func GetUUID() string {
	uid := uuid.NewV4()
	return uid.String()
}

func clearTempDir(path string) error {
	fmt.Printf("删除 temp 目录及其所有内容")
	// 删除 temp 目录及其所有内容
	return os.RemoveAll(path)
}

type PageData struct {
	TempPath string
	Result   string
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	// 获取member参数的值
	member := r.FormValue("member")
	fmt.Println("member参数的值:", member)

	uuid := GetUUID()

	tempPath := "./temp/" + uuid
	//tempPath := "./temp/" + uuid
	//defer clearTempDir(tempPath)

	// 清理 temp 目录
	err := clearTempDir(tempPath)
	if err != nil {
		http.Error(w, "清理目录出错", http.StatusInternalServerError)
		return
	}

	// 解析表单
	err = r.ParseMultipartForm(10 << 20) // 限制上传文件大小为10MB
	if err != nil {
		http.Error(w, "解析表单出错", http.StatusInternalServerError)
		return
	}
	//临时保存pdf
	saveFilesToTemp(tempPath, w, r)

	//fmt.Fprintf(w, "zip 下载链接：http://localhost:8889/download?directory=%v \n", tempPath)
	//收集转换pdf
	resultToShow := genNewPdf(member, tempPath)

	// 构建PageData结构体
	data := PageData{
		TempPath: tempPath,
		Result:   resultToShow,
	}
	// 加载HTML模板文件
	tmpl, err := template.ParseFiles("result.html")
	if err != nil {
		http.Error(w, "加载HTML模板文件出错", http.StatusInternalServerError)
		return
	}

	// 执行HTML模板文件并将结果写入HTTP响应体
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "执行HTML模板文件出错", http.StatusInternalServerError)
		return
	}

}

func zipFiles(directory string, w io.Writer) error {
	// 创建一个新的 zip 文件
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// 遍历指定目录下的所有文件和子目录
	err := filepath.Walk(directory, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取文件相对于指定目录的相对路径
		relPath, err := filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}

		// 如果是目录，则创建 zip 文件中的一个目录条目
		if info.IsDir() {
			_, err = zipWriter.Create(relPath + "/")
			return err
		}

		// 创建 zip 文件中的一个文件条目
		fileWriter, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// 打开文件
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		// 将文件内容写入 zip 文件
		_, err = io.Copy(fileWriter, file)
		return err
	})

	return err
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// 获取当前时间
	now := time.Now()

	// 获取当前月份和日期
	year := now.Year()
	month := now.Month()
	//day := now.Day()

	fileName := fmt.Sprintf("成都 - %d年%02d月 - 下午茶报销.zip", year, month)
	// 获取 GET 请求中的 directory 参数
	directory := r.URL.Query().Get("directory")

	if directory == "" {
		http.Error(w, "缺少目录参数", http.StatusBadRequest)
		return
	}

	defer func(path string) {
		time.AfterFunc(10*time.Minute, func() {
			err := clearTempDir(path)
			if err != nil {
				http.Error(w, "清理目录出错", http.StatusInternalServerError)
			}
		})
	}(directory)

	// 设置 Content-Type 为 zip 文件
	w.Header().Set("Content-Type", "application/zip")
	// 设置 Content-Disposition，让浏览器下载文件而不是直接打开
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))

	// 将指定目录下的所有文件和子目录打包成 zip 文件，并写入到 HTTP 响应体中
	err := zipFiles(directory, w)
	if err != nil {
		http.Error(w, "打包文件出错", http.StatusInternalServerError)
		return
	}
}

func saveFilesToTemp(path string, w http.ResponseWriter, r *http.Request) {
	// 获取上传的文件
	files := r.MultipartForm.File["pdfFiles"]
	for _, fileHeader := range files {
		// 打开上传的文件
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "打开上传文件出错", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			http.Error(w, "创建目录出错", http.StatusInternalServerError)
			return
		}

		// 生成目标文件路径
		targetFile := filepath.Join(path, fileHeader.Filename)

		// 创建目标文件
		f, err := os.Create(targetFile)
		if err != nil {
			http.Error(w, "创建文件出错", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		// 将上传的文件内容拷贝到目标文件中
		_, err = io.Copy(f, file)
		if err != nil {
			http.Error(w, "保存文件出错", http.StatusInternalServerError)
			return
		}

		// 提示上传成功
		//fmt.Fprintf(w, "文件 %s 上传成功，保存到：%s\n", fileHeader.Filename, targetFile)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// 读取HTML模板文件
	tmpl, err := template.ParseFiles("upload.html")
	if err != nil {
		http.Error(w, "读取HTML模板出错", http.StatusInternalServerError)
		return
	}

	// 渲染HTML模板
	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "渲染HTML模板出错", http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/download", downloadHandler)

	fmt.Println("服务器已启动，监听端口 8889...")
	err := http.ListenAndServe(":8889", nil)
	if err != nil {
		fmt.Println("服务器启动失败:", err)
		return
	}

}

func genNewPdf(customMembers, dir string) string {

	var resultToShow strings.Builder

	members := defaultMembers

	//dir = filepath.Join(dir, "/temp")
	if len(customMembers) > 0 {
		members = customMembers
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("读取目录出错:", err)
		return ""
	}
	dateResultsMap := make(map[string][]*PdfResult)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".pdf") {
			continue
		}
		filePath := filepath.Join(dir, file.Name())
		pdfResult := parseToPdfResult(filePath)
		if pdfResult == nil {
			continue
		}
		dateResultsMap[pdfResult.date] = append(dateResultsMap[pdfResult.date], pdfResult)
	}

	var totalAmounts float64

	resultToShow.WriteString(fmt.Sprint("=====以下内容可以直接写入Approval=====\n\n"))
	for dataResult, results := range dateResultsMap {
		var folderName string
		totalAmount := 0.0
		for _, result := range results {
			totalAmount += result.amount
		}
		totalAmounts += totalAmount
		folderName = fmt.Sprintf("%v/下午茶%v*%.2f元", dir, dataResult, totalAmount)

		resultToShow.WriteString("=====下午茶=====\n")
		resultToShow.WriteString(fmt.Sprintf("日期：%s\n", dataResult))
		resultToShow.WriteString(fmt.Sprintf("人员：%s\n", members))
		resultToShow.WriteString(fmt.Sprintf("当日总金额：%.2f\n", totalAmount))
		for _, result := range results {
			var fileName string
			if result.isLogistics {
				fileName = fmt.Sprintf("%v-下午茶配送费(%v)*%.2f元.pdf", result.date, result.invoiceNumber, result.amount)
			} else {
				fileName = fmt.Sprintf("%v-下午茶(%v)*%.2f元.pdf", result.date, result.invoiceNumber, result.amount)
			}
			result.newPath = filepath.Join(folderName, fileName)
			result.newFolderName = folderName
			if result.isLogistics {
				resultToShow.WriteString(fmt.Sprintf("-(配送费)金额：%.2f\n", result.amount))
				resultToShow.WriteString(fmt.Sprintf("-(配送费)发票号码：%s\n", result.invoiceNumber))
			} else {
				resultToShow.WriteString(fmt.Sprintf("-(下午茶)金额：%.2f\n", result.amount))
				resultToShow.WriteString(fmt.Sprintf("-(下午茶)发票号码：%s\n", result.invoiceNumber))
			}
		}
		resultToShow.WriteString("================\n\n")
	}
	resultToShow.WriteString(fmt.Sprintf("共计金额: %.2f \n", totalAmounts))
	resultToShow.WriteString("================\n\n")
	for _, results := range dateResultsMap {
		for _, result := range results {
			// 如果目标目录不存在，则创建它
			if err := os.MkdirAll(result.newFolderName, os.ModePerm); err != nil {
				fmt.Println("创建目标目录出错:", err)
				continue
			}

			// 重命名并移动文件
			if err := os.Rename(result.oldPath, result.newPath); err != nil {
				fmt.Println("重命名并移动文件出错:", err)
				continue
			}

		}
	}
	return resultToShow.String()
}

func parseToPdfResult(path string) *PdfResult {

	err := os.Chmod(path, 0777)
	if err != nil {
		fmt.Printf("Error Chmod PDF: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Read PDF file.
	pdfReader, err := model.NewPdfReader(f)
	if err != nil {
		fmt.Printf("Error reading PDF: %v\n", err)
		os.Exit(1)
	}

	pdfPager, err := pdfReader.GetPage(1)
	if err != nil {
		fmt.Printf("Error pdfReader PDF: %v\n", err)
		os.Exit(1)
	}

	// Extract text from PDF.
	invoceExtractor, err := extractor.New(pdfPager)
	if err != nil {
		fmt.Printf("Error creating extractor: %v\n", err)
		os.Exit(1)
	}

	text, err := invoceExtractor.ExtractText()
	if err != nil {
		fmt.Printf("Error extracting text: %v\n", err)
		os.Exit(1)
	}

	//text := "电⼦发票（普通发票） 统 一 发 票 监 制\n国\n章\n全\n国家税务总局\n发票号码： 2451200000004\n开票日期： 2024年03月22日\n四 川\n省 税 务 局\n4.36\n¥4.36\n购\n买\n⽅\n信\n息 统⼀社会信用代码/纳税⼈识别号：\n91310000062564047N\n名称：优倍快网络技术咨询（上海）有限公司 \t 销\n售\n⽅\n信\n息 统⼀社会信用代码/纳税⼈识别号：\n91510100MA68J0MK2P\n名称：成都市台盖餐饮管理有限公司府城大道第一分公司\n项目名称 规格型号 单 位 数 量 单 价 ⾦ 额 税率/征收率 税 额 \t \t \t \t\n*物流辅助服务*配送服 172.6415094339623 72.64 6% \t \t \t \t\n¥72.64\n合 计 \t \t \t \t\n价税合计（⼤写） \t 柒拾柒圆整 （小写） ¥ 77.01 \t \t\n备 \t \t \t \t\n注 \t \t \t \t\n开票⼈：李薇\n下载次数：1"
	//dateAmountMap := make(map[string]float64)
	// Regular expressions for matching patterns.
	invoiceNumberRegex := regexp.MustCompile(`发票号码：\s*(\d+)`)
	dateRegex := regexp.MustCompile(`开票日期：\s*(\d{4})\s*年\s*(\d{2})\s*月\s*(\d{2})\s*日`)
	amountRegex := regexp.MustCompile(`（小写）\s*¥\s*(\d+(\.\d+)?)`)

	var isLogistics bool
	if strings.Contains(text, "*物流辅助服务*") {
		isLogistics = true
	}
	// Find matches in the extracted text.
	invoiceNumberMatches := invoiceNumberRegex.FindStringSubmatch(text)
	amountMatches := amountRegex.FindStringSubmatch(text)
	dateMatches := dateRegex.FindStringSubmatch(text)

	// Extracted values.
	var amount float64
	var date string
	var invoiceNumber string

	// Check if matches were found and extract the values.
	if len(invoiceNumberMatches) > 1 {
		invoiceNumber = strings.TrimSpace(invoiceNumberMatches[1])
	}
	if len(amountMatches) > 1 {
		amountStr := strings.TrimSpace(amountMatches[1])
		amount, err = strconv.ParseFloat(amountStr, 10)
		if err != nil {
			fmt.Printf("ParseFloat Failed：%v\n", err)
			return nil
		}
	}
	//layout := "2006-01-02"
	if len(dateMatches) > 3 {
		year := strings.TrimSpace(dateMatches[1])
		month := strings.TrimSpace(dateMatches[2])
		day := strings.TrimSpace(dateMatches[3])
		date = fmt.Sprintf("%s年%s月%s日", year, month, day)
		//date, err = time.Parse(layout, dateStr)
		//if err != nil {
		//	fmt.Printf("解析日期出错：%v\n", err)
		//	return
		//}
	}

	// Output extracted information.
	//fmt.Printf("InvoiceNumber: %v\n", invoiceNumber)
	//fmt.Printf("Amount: %.2f\n", amount)
	//fmt.Printf("Date: %s\n", date)
	//fmt.Printf("isLogistics: %v\n", isLogistics)

	pdfResult := &PdfResult{
		invoiceNumber: invoiceNumber,
		amount:        amount,
		date:          date,
		isLogistics:   isLogistics,
		//originalFile:  f,
		oldPath: path,
	}

	return pdfResult
}

type PdfResult struct {
	invoiceNumber string
	amount        float64
	date          string
	isLogistics   bool

	//originalFile  *os.File
	oldPath       string
	newPath       string
	newFolderName string
}
