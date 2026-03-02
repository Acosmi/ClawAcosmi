import sys
from PIL import Image

def remove_white_background(image_path, output_path):
    img = Image.open(image_path)
    img = img.convert("RGBA")
    
    datas = img.getdata()
    newData = []
    
    for item in datas:
        # Check if pixel is white or near white
        # The background of the logo looks mostly white, let's say (230, 230, 230) to (255, 255, 255)
        if item[0] >= 235 and item[1] >= 235 and item[2] >= 235:
            newData.append((255, 255, 255, 0)) # transparent
        else:
            newData.append(item)
            
    img.putdata(newData)
    img.save(output_path, "PNG")

if __name__ == "__main__":
    if len(sys.argv) > 2:
        remove_white_background(sys.argv[1], sys.argv[2])
