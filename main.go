package main

import (
	"fmt"
	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func init() {

}

func main() {
	apiKey := "384e92997199140a8eda3cc184ecf29ecab602d9230504e6224024f0d4315c80"

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
	fmt.Printf("License: %s\n", lk.ToString())

	// GetMeteredState freshly checks the state, contacting the licensing server.
	state, err := license.GetMeteredState()
	if err != nil {
		fmt.Printf("ERROR getting metered state: %+v\n", err)
		panic(err)
	}
	fmt.Printf("State: %+v\n", state)
	if state.OK {
		fmt.Printf("State is OK\n")
	} else {
		fmt.Printf("State is not OK\n")
	}

	// 获取当前目录
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("获取当前目录出错:", err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("读取目录出错:", err)
		return
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

	for dataResult, results := range dateResultsMap {
		var folderName string
		totalAmount := 0.0
		for _, result := range results {
			totalAmount += result.amount
		}
		folderName = fmt.Sprintf("%v/下午茶%v*%.2f元", dir, dataResult, totalAmount)

		for _, result := range results {
			var fileName string
			if result.isLogistics {
				fileName = fmt.Sprintf("%v-下午茶配送费(%v)*%.2f元.pdf", result.date, result.invoiceNumber, result.amount)
			} else {
				fileName = fmt.Sprintf("%v-下午茶(%v)*%.2f元.pdf", result.date, result.invoiceNumber, result.amount)
			}
			result.newPath = filepath.Join(folderName, fileName)
			result.newFolderName = folderName
		}
	}

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
				return
			}

		}
	}
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
	fmt.Printf("InvoiceNumber: %v\n", invoiceNumber)
	fmt.Printf("Amount: %.2f\n", amount)
	fmt.Printf("Date: %s\n", date)
	fmt.Printf("isLogistics: %v\n", isLogistics)
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
